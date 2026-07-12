package mux

import (
	"errors"
	"fmt"
	"io"
	"os"

	"cromedia/core"
	"cromedia/core/demux"
)

// ConcatMP4Files concatenates multiple MP4 files using stream copy.
func ConcatMP4Files(outputPath string, inputPaths []string) error {
	if len(inputPaths) == 0 {
		return errors.New("no input files provided for concatenation")
	}

	// 1. Open all input files and probe them
	files := make([]*os.File, len(inputPaths))
	demuxers := make([]*demux.MP4Demuxer, len(inputPaths))
	fileTracks := make([][]core.Track, len(inputPaths))

	defer func() {
		for _, f := range files {
			if f != nil {
				f.Close()
			}
		}
	}()

	for i, path := range inputPaths {
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open input file %s: %w", path, err)
		}
		files[i] = f

		d := demux.NewMP4Demuxer(f)
		tracks, err := d.Probe()
		if err != nil {
			return fmt.Errorf("failed to probe input file %s: %w", path, err)
		}
		demuxers[i] = d
		fileTracks[i] = tracks
	}

	// 2. Validate track compatibility
	firstTracks := fileTracks[0]
	numTracks := len(firstTracks)
	if numTracks == 0 {
		return errors.New("first input file contains no tracks")
	}

	for i := 1; i < len(fileTracks); i++ {
		tracks := fileTracks[i]
		if len(tracks) != numTracks {
			return fmt.Errorf("track count mismatch: file %s has %d tracks, expected %d", inputPaths[i], len(tracks), numTracks)
		}
		for tIdx := 0; tIdx < numTracks; tIdx++ {
			t1 := firstTracks[tIdx]
			t2 := tracks[tIdx]
			if t1.Type != t2.Type {
				return fmt.Errorf("track %d type mismatch: file %s has type %s, expected %s", tIdx, inputPaths[i], t2.Type, t1.Type)
			}
			if t1.CodecTag != t2.CodecTag {
				return fmt.Errorf("track %d codec tag mismatch: file %s has '%s', expected '%s'", tIdx, inputPaths[i], t2.CodecTag, t1.CodecTag)
			}
			if t1.Timescale != t2.Timescale {
				return fmt.Errorf("track %d timescale mismatch: file %s has %d, expected %d", tIdx, inputPaths[i], t2.Timescale, t1.Timescale)
			}
		}
	}

	// 3. Construct merged track descriptions
	mergedTracks := make([]core.Track, numTracks)
	for tIdx := 0; tIdx < numTracks; tIdx++ {
		// Copy metadata structure from the first file's track
		mergedTracks[tIdx] = firstTracks[tIdx]
		// Reset samples & ctts offsets to build them up
		mergedTracks[tIdx].Samples = nil
		mergedTracks[tIdx].CTSOffsets = nil
	}

	// Map each sample of the merged tracks back to its source file and original sample index
	// sampleSources[trackIndex][sampleIndex] -> {fileIndex, originalSampleIndex}
	type sampleSourceInfo struct {
		fileIdx   int
		sampleIdx int
	}
	var sampleSources [][]sampleSourceInfo = make([][]sampleSourceInfo, numTracks)

	// Keep track of the running DTS offset for each track
	dtsOffsets := make([]int64, numTracks)

	for fileIdx := 0; fileIdx < len(inputPaths); fileIdx++ {
		tracks := fileTracks[fileIdx]
		for tIdx := 0; tIdx < numTracks; tIdx++ {
			t := tracks[tIdx]

			// Determine DTS offset from the previous file's samples
			dtsOffset := dtsOffsets[tIdx]

			// Append samples
			startSampleID := len(mergedTracks[tIdx].Samples) + 1
			for sIdx, s := range t.Samples {
				newSample := s
				newSample.ID = startSampleID + sIdx
				newSample.Time = s.Time + dtsOffset
				mergedTracks[tIdx].Samples = append(mergedTracks[tIdx].Samples, newSample)

				// Keep track of the source of this sample
				sampleSources[tIdx] = append(sampleSources[tIdx], sampleSourceInfo{
					fileIdx:   fileIdx,
					sampleIdx: sIdx,
				})
			}

			// Append composition offsets (ctts) if present
			mergedTracks[tIdx].CTSOffsets = append(mergedTracks[tIdx].CTSOffsets, t.CTSOffsets...)

			// Calculate next DTS offset
			if len(t.Samples) > 0 {
				lastSample := t.Samples[len(t.Samples)-1]
				dtsOffsets[tIdx] += lastSample.Time + lastSample.Duration
			}
		}
	}

	// Calculate and update final duration for each track
	for tIdx := 0; tIdx < numTracks; tIdx++ {
		var totalDur uint64 = 0
		for _, s := range mergedTracks[tIdx].Samples {
			totalDur += uint64(s.Duration)
		}
		mergedTracks[tIdx].Duration = totalDur
	}

	// 4. Create output file and setup MP4Muxer
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	muxer := NewMP4Muxer(out)
	err = muxer.WriteHeader(mergedTracks)
	if err != nil {
		return fmt.Errorf("failed to write MP4 header: %w", err)
	}

	// 5. Interleave samples and copy raw payload data
	// The order of writing to mdat must match BuildInterleavedOrder(mergedTracks)
	interleaved := BuildInterleavedOrder(mergedTracks)

	// Pre-allocate a 64KB copy buffer from BufferPool
	bufSize := 65536
	copyBuf := core.GlobalGet(bufSize)
	defer core.GlobalPut(copyBuf)

	for _, is := range interleaved {
		// Find source file and sample index
		srcInfo := sampleSources[is.TrackIndex][is.SampleIndex]
		srcFile := files[srcInfo.fileIdx]
		
		// Get original sample from the probed tracks
		origSample := fileTracks[srcInfo.fileIdx][is.TrackIndex].Samples[srcInfo.sampleIdx]

		// Seek to the original sample offset in the source file
		_, err = srcFile.Seek(origSample.Offset, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek in source file %d at offset %d: %w", srcInfo.fileIdx, origSample.Offset, err)
		}

		// Read and write the sample payload in chunks
		remaining := origSample.Size
		for remaining > 0 {
			toRead := int64(bufSize)
			if remaining < toRead {
				toRead = remaining
			}

			_, err = io.ReadFull(srcFile, copyBuf[:toRead])
			if err != nil {
				return fmt.Errorf("failed to read payload from source file %d: %w", srcInfo.fileIdx, err)
			}

			pkt := &core.Packet{Data: copyBuf[:toRead]}
			err = muxer.WritePacket(pkt)
			if err != nil {
				return fmt.Errorf("failed to write sample packet: %w", err)
			}
			remaining -= toRead
		}
	}

	err = muxer.WriteTrailer()
	if err != nil {
		return fmt.Errorf("failed to write trailer: %w", err)
	}

	return nil
}
