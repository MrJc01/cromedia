package demux

import (
	"fmt"
	"io"
	"os"

	"cromedia/core"
)

// AnnexBDemuxer demuxes raw Annex B byte streams (e.g. H.264).
type AnnexBDemuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
}

// NewAnnexBDemuxer instantiates a new AnnexBDemuxer.
func NewAnnexBDemuxer(file *os.File) *AnnexBDemuxer {
	return &AnnexBDemuxer{file: file}
}

// Probe parses start codes and indexes NAL units.
func (d *AnnexBDemuxer) Probe() ([]core.Track, error) {
	info, err := d.file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	// Read entire stream or read buffer-by-buffer. Since we need to parse offsets,
	// let's do a simple buffer scanner.
	buf := make([]byte, 4096)
	var fileOffset int64 = 0

	type nalLoc struct {
		offset int64
		size   int64
		isKey  bool
	}
	var nals []nalLoc

	var lastStartCodeOffset int64 = -1
	var lastNalType byte = 0

	for fileOffset < fileSize {
		if _, err := d.file.Seek(fileOffset, io.SeekStart); err != nil {
			break
		}
		n, err := d.file.Read(buf)
		if err != nil && err != io.EOF {
			break
		}
		if n == 0 {
			break
		}

		for i := 0; i < n; i++ {
			currPos := fileOffset + int64(i)
			// Check for 3-byte or 4-byte start codes
			var isStartCode = false
			var headerLen int64 = 0

			if currPos+3 <= fileSize && buf[i] == 0x00 && buf[i+1] == 0x00 && buf[i+2] == 0x01 {
				isStartCode = true
				headerLen = 3
			} else if currPos+4 <= fileSize && buf[i] == 0x00 && buf[i+1] == 0x00 && buf[i+2] == 0x00 && buf[i+3] == 0x01 {
				isStartCode = true
				headerLen = 4
			}

			if isStartCode {
				if lastStartCodeOffset != -1 {
					size := currPos - lastStartCodeOffset
					if size > 0 {
						nals = append(nals, nalLoc{
							offset: lastStartCodeOffset,
							size:   size,
							isKey:  lastNalType == 5 || lastNalType == 7 || lastNalType == 8,
						})
					}
				}
				lastStartCodeOffset = currPos
				// Peak NAL type byte
				var nalTypeByte [1]byte
				if _, err := d.file.ReadAt(nalTypeByte[:], currPos+headerLen); err == nil {
					lastNalType = nalTypeByte[0] & 0x1F
				}
				// Skip start code bytes to prevent double detection
				i += int(headerLen) - 1
			}
		}

		fileOffset += int64(n)
		// Small overlap to make sure we don't miss start codes spanning across read chunks
		if fileOffset < fileSize {
			fileOffset -= 4
		}
	}

	// Add final NAL
	if lastStartCodeOffset != -1 {
		size := fileSize - lastStartCodeOffset
		if size > 0 {
			nals = append(nals, nalLoc{
				offset: lastStartCodeOffset,
				size:   size,
				isKey:  lastNalType == 5 || lastNalType == 7 || lastNalType == 8,
			})
		}
	}

	if len(nals) == 0 {
		return nil, fmt.Errorf("no NAL units found in Annex B stream")
	}

	track := core.Track{
		ID:        1,
		Type:      core.TrackTypeVideo,
		Timescale: 90000,
		CodecTag:  "h264",
	}

	var pts int64 = 0
	for i, n := range nals {
		sample := core.Sample{
			ID:         i + 1,
			IsKeyframe: n.isKey,
			Offset:     n.offset,
			Size:       n.size,
			Time:       pts,
			Duration:   3000, // Assumed 30fps (3000 units in 90k timescale)
		}
		track.Samples = append(track.Samples, sample)
		pts += 3000
	}

	track.Duration = uint64(pts)
	d.tracks = []core.Track{track}

	d.interleaved = make([]core.InterleavedSample, len(track.Samples))
	for i, s := range track.Samples {
		d.interleaved[i] = core.InterleavedSample{
			TrackIndex:  0,
			SampleIndex: i,
			TimeSeconds: float64(s.Time) / 90000.0,
			Sample:      s,
		}
	}
	d.currentSample = 0

	return d.tracks, nil
}

// ReadPacket returns the next interleaved packet.
func (d *AnnexBDemuxer) ReadPacket() (*core.Packet, error) {
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
func (d *AnnexBDemuxer) Close() error {
	return d.file.Close()
}
