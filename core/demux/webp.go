package demux

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"cromedia/core"
)

// WebPDemuxer handles demuxing of animated WebP files.
type WebPDemuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
}

// NewWebPDemuxer instantiates a new WebPDemuxer.
func NewWebPDemuxer(file *os.File) *WebPDemuxer {
	return &WebPDemuxer{file: file}
}

// Probe parses RIFF chunks and maps ANMF blocks to samples.
func (d *WebPDemuxer) Probe() ([]core.Track, error) {
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

	if string(riffHeader[0:4]) != "RIFF" || string(riffHeader[8:12]) != "WEBP" {
		return nil, fmt.Errorf("invalid RIFF/WEBP signature")
	}

	track := core.Track{
		ID:        1,
		Type:      core.TrackTypeVideo,
		Timescale: 1000, // timescale is 1000 for milliseconds durations
		CodecTag:  "webp",
	}

	var sampleTime int64 = 0
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

		if chunkID == "ANMF" {
			if chunkSize < 16 {
				offset += 8 + chunkSize + (chunkSize % 2)
				continue
			}

			anmfMeta := make([]byte, 16)
			if _, err := io.ReadFull(d.file, anmfMeta); err != nil {
				break
			}

			// Frame duration: 3 bytes at offset 12 of ANMF (i.e. anmfMeta[12:15])
			duration := int64(anmfMeta[12]) | int64(anmfMeta[13])<<8 | int64(anmfMeta[14])<<16

			sample := core.Sample{
				ID:         len(track.Samples) + 1,
				IsKeyframe: true,
				Offset:     offset + 8 + 16,
				Size:       chunkSize - 16,
				Time:       sampleTime,
				Duration:   duration,
			}
			track.Samples = append(track.Samples, sample)

			sampleTime += duration
		}

		offset += 8 + chunkSize + (chunkSize % 2)
	}

	if len(track.Samples) == 0 {
		return nil, fmt.Errorf("no animated frames (ANMF) found in WebP")
	}

	track.Duration = uint64(sampleTime)
	d.tracks = []core.Track{track}

	d.interleaved = make([]core.InterleavedSample, len(track.Samples))
	for i, s := range track.Samples {
		d.interleaved[i] = core.InterleavedSample{
			TrackIndex:  0,
			SampleIndex: i,
			TimeSeconds: float64(s.Time) / 1000.0,
			Sample:      s,
		}
	}
	d.currentSample = 0

	return d.tracks, nil
}

// ReadPacket returns the next interleaved packet.
func (d *WebPDemuxer) ReadPacket() (*core.Packet, error) {
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
func (d *WebPDemuxer) Close() error {
	return d.file.Close()
}
