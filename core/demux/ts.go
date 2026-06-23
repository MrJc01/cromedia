package demux

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"

	"cromedia/core"
)

// TSDemuxer handles demuxing of MPEG-TS (.ts) files.
type TSDemuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
}

// NewTSDemuxer instantiates a new TSDemuxer.
func NewTSDemuxer(file *os.File) *TSDemuxer {
	return &TSDemuxer{file: file}
}

type tsTrackInfo struct {
	PID      uint16
	Type     core.TrackType
	CodecTag string
}

// Probe parses PAT and PMT tables, mappings PES packets and generating track info.
func (d *TSDemuxer) Probe() ([]core.Track, error) {
	info, err := d.file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	var pmtPID uint16 = 0
	var tracksInfo []tsTrackInfo
	trackMap := make(map[uint16]*core.Track)

	type pesAccumulator struct {
		data        []byte
		startOffset int64
	}
	accumulators := make(map[uint16]*pesAccumulator)

	finalizePES := func(pid uint16) {
		acc, exists := accumulators[pid]
		if !exists || len(acc.data) == 0 {
			return
		}
		tObj, hasTrack := trackMap[pid]
		if !hasTrack {
			return
		}

		pts, dts, headerLen, err := parsePESHeader(acc.data)
		if err == nil && len(acc.data) > headerLen {
			isKey := true
			if tObj.Type == core.TrackTypeVideo {
				isKey = hasH264Keyframe(acc.data[headerLen:])
			}

			sample := core.Sample{
				ID:         len(tObj.Samples) + 1,
				IsKeyframe: isKey,
				Offset:     acc.startOffset + int64(headerLen),
				Size:       int64(len(acc.data) - headerLen),
				Time:       dts,
				Duration:   0,
			}

			if pts != dts {
				for len(tObj.CTSOffsets) < len(tObj.Samples) {
					tObj.CTSOffsets = append(tObj.CTSOffsets, 0)
				}
				tObj.CTSOffsets = append(tObj.CTSOffsets, int32(pts-dts))
			}

			if len(tObj.Samples) > 0 {
				prevIdx := len(tObj.Samples) - 1
				tObj.Samples[prevIdx].Duration = dts - tObj.Samples[prevIdx].Time
			}
			tObj.Samples = append(tObj.Samples, sample)
		}
		delete(accumulators, pid)
	}

	buf := make([]byte, 188)
	var offset int64 = 0

	for offset+188 <= fileSize {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}
		if _, err := io.ReadFull(d.file, buf); err != nil {
			break
		}

		if buf[0] != 0x47 {
			offset++
			continue
		}

		header := binary.BigEndian.Uint32(buf[0:4])
		pusi := (header & 0x00400000) != 0
		pid := uint16((header & 0x001FFF00) >> 8)
		adaptationFieldControl := byte((header & 0x000000C0) >> 6)

		payloadStart := 4
		if adaptationFieldControl == 2 || adaptationFieldControl == 3 {
			afLen := int(buf[4])
			payloadStart = 5 + afLen
		}

		if payloadStart >= 188 {
			offset += 188
			continue
		}

		payload := buf[payloadStart:188]
		payloadFileOffset := offset + int64(payloadStart)

		if pid == 0 { // PAT
			if pusi && len(payload) > 1 {
				ptr := int(payload[0])
				if ptr+1 < len(payload) {
					pidVal, err := parsePAT(payload[1+ptr:])
					if err == nil {
						pmtPID = pidVal
					}
				}
			}
		} else if pmtPID != 0 && pid == pmtPID { // PMT
			if pusi && len(payload) > 1 {
				ptr := int(payload[0])
				if ptr+1 < len(payload) {
					tInfos, err := parsePMT(payload[1+ptr:])
					if err == nil && len(tracksInfo) == 0 {
						tracksInfo = tInfos
						for _, info := range tracksInfo {
							tr := core.Track{
								ID:        int(info.PID),
								Type:      info.Type,
								Timescale: 90000,
								CodecTag:  info.CodecTag,
							}
							d.tracks = append(d.tracks, tr)
						}
						for i := range d.tracks {
							trackMap[uint16(d.tracks[i].ID)] = &d.tracks[i]
						}
					}
				}
			}
		} else {
			_, hasTrack := trackMap[pid]
			if hasTrack {
				if pusi {
					finalizePES(pid)
					accumulators[pid] = &pesAccumulator{
						data:        append([]byte{}, payload...),
						startOffset: payloadFileOffset,
					}
				} else {
					acc, exists := accumulators[pid]
					if exists {
						acc.data = append(acc.data, payload...)
					}
				}
			}
		}

		offset += 188
	}

	for pid := range accumulators {
		finalizePES(pid)
	}

	for i := range d.tracks {
		t := &d.tracks[i]
		if len(t.Samples) > 0 {
			if len(t.Samples) > 1 {
				t.Samples[len(t.Samples)-1].Duration = t.Samples[len(t.Samples)-2].Duration
			} else {
				t.Samples[len(t.Samples)-1].Duration = 3000 // ~30 fps default duration (90k timescale)
			}
			t.Duration = uint64(t.Samples[len(t.Samples)-1].Time + t.Samples[len(t.Samples)-1].Duration)
		}
	}

	d.interleaved = d.buildInterleavedSamples()
	d.currentSample = 0
	return d.tracks, nil
}

// ReadPacket returns the next interleaved packet.
func (d *TSDemuxer) ReadPacket() (*core.Packet, error) {
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
func (d *TSDemuxer) Close() error {
	return d.file.Close()
}

func (d *TSDemuxer) buildInterleavedSamples() []core.InterleavedSample {
	var all []core.InterleavedSample
	for ti, t := range d.tracks {
		ts := float64(t.Timescale)
		if ts == 0 {
			ts = 90000
		}
		for si, s := range t.Samples {
			timeSeconds := float64(s.Time) / ts
			all = append(all, core.InterleavedSample{
				TrackIndex:  ti,
				SampleIndex: si,
				TimeSeconds: timeSeconds,
				Sample:      s,
			})
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		if all[i].TimeSeconds != all[j].TimeSeconds {
			return all[i].TimeSeconds < all[j].TimeSeconds
		}
		return all[i].TrackIndex < all[j].TrackIndex
	})

	return all
}

func parsePAT(payload []byte) (pmtPID uint16, err error) {
	if len(payload) < 8 {
		return 0, fmt.Errorf("PAT payload too short")
	}
	tableID := payload[0]
	if tableID != 0x00 {
		return 0, fmt.Errorf("invalid PAT table_id: 0x%02x", tableID)
	}

	sectionLength := uint16(payload[1]&0x0F)<<8 | uint16(payload[2])
	if len(payload) < int(sectionLength+3) {
		return 0, fmt.Errorf("PAT section length exceeds payload")
	}

	loopEnd := int(sectionLength + 3 - 4)
	for i := 8; i < loopEnd; i += 4 {
		programNum := binary.BigEndian.Uint16(payload[i : i+2])
		pmtPIDVal := binary.BigEndian.Uint16(payload[i+2 : i+4]) & 0x1FFF
		if programNum != 0 {
			return pmtPIDVal, nil
		}
	}

	return 0, fmt.Errorf("no program mapping found in PAT")
}

func parsePMT(payload []byte) (tracks []tsTrackInfo, err error) {
	if len(payload) < 12 {
		return nil, fmt.Errorf("PMT payload too short")
	}
	tableID := payload[0]
	if tableID != 0x02 {
		return nil, fmt.Errorf("invalid PMT table_id: 0x%02x", tableID)
	}

	sectionLength := uint16(payload[1]&0x0F)<<8 | uint16(payload[2])
	if len(payload) < int(sectionLength+3) {
		return nil, fmt.Errorf("PMT section length exceeds payload")
	}

	programInfoLength := uint16(payload[10]&0x0F)<<8 | uint16(payload[11])
	idx := 12 + int(programInfoLength)

	loopEnd := int(sectionLength + 3 - 4)
	for idx < loopEnd {
		if idx+5 > loopEnd {
			break
		}
		streamType := payload[idx]
		elemPID := binary.BigEndian.Uint16(payload[idx+1:idx+3]) & 0x1FFF
		esInfoLength := uint16(payload[idx+3]&0x0F)<<8 | uint16(payload[idx+4])

		var trackType core.TrackType
		var codecTag string
		switch streamType {
		case 0x1B: // H.264
			trackType = core.TrackTypeVideo
			codecTag = "avc1"
		case 0x24: // HEVC
			trackType = core.TrackTypeVideo
			codecTag = "hev1"
		case 0x0F: // AAC
			trackType = core.TrackTypeAudio
			codecTag = "mp4a"
		case 0x03, 0x04: // MP3
			trackType = core.TrackTypeAudio
			codecTag = "mp3"
		case 0x81, 0x82, 0x87: // AC3
			trackType = core.TrackTypeAudio
			codecTag = "ac-3"
		default:
			trackType = core.TrackTypeMeta
		}

		if trackType != core.TrackTypeMeta {
			tracks = append(tracks, tsTrackInfo{
				PID:      elemPID,
				Type:     trackType,
				CodecTag: codecTag,
			})
		}

		idx += 5 + int(esInfoLength)
	}

	return tracks, nil
}

func parsePESHeader(payload []byte) (pts, dts int64, headerLen int, err error) {
	if len(payload) < 6 {
		return 0, 0, 0, fmt.Errorf("PES header too short")
	}
	prefix := uint32(payload[0])<<16 | uint32(payload[1])<<8 | uint32(payload[2])
	if prefix != 0x000001 {
		return 0, 0, 0, fmt.Errorf("invalid PES prefix: 0x%06x", prefix)
	}

	streamID := payload[3]
	if streamID != 0xBC && streamID != 0xBE && streamID != 0xBF && streamID != 0xF0 && streamID != 0xF1 && streamID != 0xFF && streamID != 0xF2 && streamID != 0xF8 {
		if len(payload) < 9 {
			return 0, 0, 0, fmt.Errorf("PES optional header too short")
		}
		ptsDtsFlags := (payload[7] & 0xC0) >> 6
		headerDataLen := payload[8]
		headerLen = 9 + int(headerDataLen)

		pts = -1
		dts = -1

		if ptsDtsFlags == 2 {
			if len(payload) >= 14 {
				pts = parseTimestamp(payload[9:14])
				dts = pts
			}
		} else if ptsDtsFlags == 3 {
			if len(payload) >= 19 {
				pts = parseTimestamp(payload[9:14])
				dts = parseTimestamp(payload[14:19])
			}
		}
	} else {
		headerLen = 6
	}

	return pts, dts, headerLen, nil
}

func parseTimestamp(b []byte) int64 {
	val := int64(b[0]&0x0E) << 29
	val |= int64(b[1]) << 22
	val |= int64(b[2]&0xFE) << 14
	val |= int64(b[3]) << 7
	val |= int64(b[4] >> 1)
	return val
}

func hasH264Keyframe(data []byte) bool {
	for i := 0; i < len(data)-4; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			nalType := data[i+3] & 0x1F
			if nalType == 5 || nalType == 7 || nalType == 8 {
				return true
			}
		} else if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x01 {
			if i+4 < len(data) {
				nalType := data[i+4] & 0x1F
				if nalType == 5 || nalType == 7 || nalType == 8 {
					return true
				}
			}
		}
	}
	return false
}
