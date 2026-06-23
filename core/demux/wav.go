package demux

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"cromedia/core"
)

// WAVDemuxer handles demuxing of RIFF/WAV audio files.
type WAVDemuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
}

// NewWAVDemuxer instantiates a new WAVDemuxer.
func NewWAVDemuxer(file *os.File) *WAVDemuxer {
	return &WAVDemuxer{file: file}
}

// Probe parses the WAV header, fmt and data chunks.
func (d *WAVDemuxer) Probe() ([]core.Track, error) {
	info, err := d.file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	riffHeader := make([]byte, 12)
	if _, err := io.ReadFull(d.file, riffHeader); err != nil {
		return nil, fmt.Errorf("failed to read RIFF header: %w", err)
	}

	if string(riffHeader[0:4]) != "RIFF" || string(riffHeader[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a valid RIFF/WAVE file")
	}

	var sampleRate uint32
	var blockAlign uint16
	var channels uint16
	var dataOffset int64
	var dataSize int64

	var offset int64 = 12
	for offset+8 <= fileSize {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}

		chunkHdr := make([]byte, 8)
		if _, err := io.ReadFull(d.file, chunkHdr); err != nil {
			break
		}

		chunkID := string(chunkHdr[0:4])
		chunkSize := int64(binary.LittleEndian.Uint32(chunkHdr[4:8]))

		if chunkID == "fmt " {
			fmtData := make([]byte, 16)
			if _, err := io.ReadFull(d.file, fmtData); err != nil {
				return nil, fmt.Errorf("failed to read fmt chunk: %w", err)
			}
			channels = binary.LittleEndian.Uint16(fmtData[2:4])
			_ = channels
			sampleRate = binary.LittleEndian.Uint32(fmtData[4:8])
			blockAlign = binary.LittleEndian.Uint16(fmtData[12:14])
		} else if chunkID == "data" {
			dataOffset = offset + 8
			dataSize = chunkSize
			break // Usually data is the last chunk, or we can just proceed.
		}

		offset += 8 + chunkSize
		// Pad byte if size is odd
		if chunkSize%2 != 0 {
			offset++
		}
	}

	if sampleRate == 0 || blockAlign == 0 || dataOffset == 0 {
		return nil, fmt.Errorf("missing fmt or data chunk in WAV")
	}

	track := core.Track{
		ID:        1,
		Type:      core.TrackTypeAudio,
		Timescale: sampleRate,
		CodecTag:  "lpcm",
		Volume:    256,
	}

	// Split data chunk into packets of 1024 samples
	const samplesPerPacket = 1024
	packetSizeBytes := int64(samplesPerPacket) * int64(blockAlign)

	var sampleOffset int64 = 0
	var byteOffset = dataOffset

	for byteOffset < dataOffset+dataSize {
		remainingBytes := dataOffset + dataSize - byteOffset
		curPacketBytes := packetSizeBytes
		if remainingBytes < packetSizeBytes {
			curPacketBytes = remainingBytes
		}

		curSamples := curPacketBytes / int64(blockAlign)
		if curSamples == 0 {
			break
		}

		sample := core.Sample{
			ID:         len(track.Samples) + 1,
			IsKeyframe: true,
			Offset:     byteOffset,
			Size:       curPacketBytes,
			Time:       sampleOffset,
			Duration:   curSamples,
		}
		track.Samples = append(track.Samples, sample)

		sampleOffset += curSamples
		byteOffset += curPacketBytes
	}

	track.Duration = uint64(sampleOffset)
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
func (d *WAVDemuxer) ReadPacket() (*core.Packet, error) {
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
func (d *WAVDemuxer) Close() error {
	return d.file.Close()
}
