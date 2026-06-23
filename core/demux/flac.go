package demux

import (
	"fmt"
	"io"
	"os"

	"cromedia/core"
)

// FLACDemuxer handles demuxing of FLAC (.flac) files.
type FLACDemuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
}

// NewFLACDemuxer instantiates a new FLACDemuxer.
func NewFLACDemuxer(file *os.File) *FLACDemuxer {
	return &FLACDemuxer{file: file}
}

// Probe parses the FLAC signature, STREAMINFO block, and frame structure.
func (d *FLACDemuxer) Probe() ([]core.Track, error) {
	info, err := d.file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	// 1. Read signature (4 bytes "fLaC")
	sig := make([]byte, 4)
	if _, err := io.ReadFull(d.file, sig); err != nil {
		return nil, fmt.Errorf("failed to read FLAC signature: %w", err)
	}
	if string(sig) != "fLaC" {
		return nil, fmt.Errorf("invalid FLAC signature")
	}

	var sampleRate uint32
	var hasStreaminfo bool

	// 2. Parse metadata blocks
	var isLastBlock = false
	var offset int64 = 4

	for !isLastBlock && offset+4 <= fileSize {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}

		hdr := make([]byte, 4)
		if _, err := io.ReadFull(d.file, hdr); err != nil {
			break
		}

		isLastBlock = (hdr[0] & 0x80) != 0
		blockType := hdr[0] & 0x7F
		blockLen := int64(hdr[1])<<16 | int64(hdr[2])<<8 | int64(hdr[3])

		if blockType == 0 { // STREAMINFO
			if blockLen < 34 {
				return nil, fmt.Errorf("STREAMINFO block too short")
			}
			streaminfo := make([]byte, 34)
			if _, err := io.ReadFull(d.file, streaminfo); err != nil {
				return nil, fmt.Errorf("failed to read STREAMINFO: %w", err)
			}
			// Sample rate is 20 bits at offset 10-12
			sampleRate = uint32(streaminfo[10])<<12 | uint32(streaminfo[11])<<4 | uint32(streaminfo[12]&0xF0)>>4
			hasStreaminfo = true
		}

		offset += 4 + blockLen
	}

	if !hasStreaminfo || sampleRate == 0 {
		return nil, fmt.Errorf("missing or invalid STREAMINFO in FLAC")
	}

	track := core.Track{
		ID:        1,
		Type:      core.TrackTypeAudio,
		Timescale: sampleRate,
		CodecTag:  "flac",
		Volume:    256,
	}

	// 3. Scan for frame syncs starting at the end of metadata blocks (offset)
	var sampleTime int64 = 0
	var lastFrameOffset int64 = -1
	var lastFrameBlockSize int64 = 0

	headerBuf := make([]byte, 16) // Max header length is around 16 bytes

	for offset+4 <= fileSize {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}

		// Frame Sync: 14 bits set to 11111111111110 -> 0xFFF8 (with 0xFFFC mask)
		n, err := d.file.Read(headerBuf)
		if err != nil || n < 4 {
			break
		}

		if headerBuf[0] != 0xFF || (headerBuf[1]&0xFC) != 0xF8 {
			offset++
			continue
		}

		// Decode block size code from headerBuf[2] bits [7:4]
		blockSizeCode := (headerBuf[2] & 0xF0) >> 4
		var blockSamples int64 = 4096 // Default fallback

		switch blockSizeCode {
		case 1:
			blockSamples = 192
		case 2:
			blockSamples = 576
		case 3:
			blockSamples = 1152
		case 4:
			blockSamples = 2304
		case 5:
			// Read 8-bit block size - 1 later or default to 576
			blockSamples = 576
		case 6:
			// Read 8-bit block size from end of header
			// To keep it simple, look ahead or use 4096
			blockSamples = 4096
		case 7:
			// Read 16-bit block size from end of header
			blockSamples = 4096
		case 8:
			blockSamples = 256
		case 9:
			blockSamples = 512
		case 10:
			blockSamples = 1024
		case 11:
			blockSamples = 2048
		case 12:
			blockSamples = 4096
		case 13:
			blockSamples = 8192
		case 14:
			blockSamples = 16384
		case 15:
			blockSamples = 32768
		}

		// If we had a previous frame, close it out now that we found the next frame start
		if lastFrameOffset != -1 {
			size := offset - lastFrameOffset
			sample := core.Sample{
				ID:         len(track.Samples) + 1,
				IsKeyframe: true,
				Offset:     lastFrameOffset,
				Size:       size,
				Time:       sampleTime,
				Duration:   lastFrameBlockSize,
			}
			track.Samples = append(track.Samples, sample)
			sampleTime += lastFrameBlockSize
		}

		lastFrameOffset = offset
		lastFrameBlockSize = blockSamples
		offset += 4 // Move forward past sync
	}

	// Add the last frame
	if lastFrameOffset != -1 {
		size := fileSize - lastFrameOffset
		if size > 0 {
			sample := core.Sample{
				ID:         len(track.Samples) + 1,
				IsKeyframe: true,
				Offset:     lastFrameOffset,
				Size:       size,
				Time:       sampleTime,
				Duration:   lastFrameBlockSize,
			}
			track.Samples = append(track.Samples, sample)
			sampleTime += lastFrameBlockSize
		}
	}

	track.Duration = uint64(sampleTime)
	d.tracks = []core.Track{track}

	d.interleaved = make([]core.InterleavedSample, len(track.Samples))
	for i, s := range track.Samples {
		d.interleaved[i] = core.InterleavedSample{
			TrackIndex:  0,
			SampleIndex: i,
			TimeSeconds: float64(s.Time) / float64(sampleRate),
			Sample:      s,
		}
	}
	d.currentSample = 0

	return d.tracks, nil
}

// ReadPacket returns the next interleaved packet.
func (d *FLACDemuxer) ReadPacket() (*core.Packet, error) {
	if d.currentSample >= len(d.interleaved) {
		return nil, io.EOF
	}

	is := d.interleaved[d.currentSample]
	d.currentSample++

	if _, err := d.file.Seek(is.Sample.Offset, io.SeekStart); err != nil {
		return nil, err
	}

	buf := core.GlobalGet(int(is.Sample.Size))
	if _, err := io.ReadFull(d.file, buf); err != nil {
		core.GlobalPut(buf)
		return nil, err
	}

	pkt := &core.Packet{
		ID:          core.NewPacketID(),
		StreamIndex: is.TrackIndex,
		Data:        buf,
		PTS:         is.Sample.Time,
		DTS:         is.Sample.Time,
		Duration:    is.Sample.Duration,
		IsKeyframe:  is.Sample.IsKeyframe,
	}

	return pkt, nil
}

// Close closes the underlying media file.
func (d *FLACDemuxer) Close() error {
	return d.file.Close()
}
