package demux

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"cromedia/core"
)

// WebMDemuxer handles demuxing of WebM and Matroska (.mkv) files.
type WebMDemuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
	timecodeScale int64 // timescale scaling in nanoseconds
}

// NewWebMDemuxer instantiates a new WebMDemuxer.
func NewWebMDemuxer(file *os.File) *WebMDemuxer {
	return &WebMDemuxer{
		file:          file,
		timecodeScale: 1000000, // Default to 1ms
	}
}

// ReadElementHeader parses an EBML ID and size.
func (d *WebMDemuxer) ReadElementHeader() (id uint32, size int64, bytesRead int, err error) {
	var b [1]byte
	if _, err := io.ReadFull(d.file, b[:]); err != nil {
		return 0, 0, 0, err
	}

	firstByte := b[0]
	var mask byte = 0x80
	length := 1
	for (firstByte & mask) == 0 {
		mask >>= 1
		length++
	}

	idVal := uint64(firstByte)
	if length > 1 {
		buf := make([]byte, length-1)
		if _, err := io.ReadFull(d.file, buf); err != nil {
			return 0, 0, 0, err
		}
		for _, v := range buf {
			idVal = (idVal << 8) | uint64(v)
		}
	}
	id = uint32(idVal)
	bytesRead = length

	szVal, szLen, err := d.readVINT()
	if err != nil {
		return 0, 0, 0, err
	}
	bytesRead += szLen

	var unknownVal uint64 = (1 << (uint(szLen) * 7)) - 1
	if szVal == unknownVal {
		size = -1
	} else {
		size = int64(szVal)
	}

	return id, size, bytesRead, nil
}

func (d *WebMDemuxer) readVINT() (uint64, int, error) {
	var b [1]byte
	if _, err := io.ReadFull(d.file, b[:]); err != nil {
		return 0, 0, err
	}

	firstByte := b[0]
	var mask byte = 0x80
	length := 1
	for (firstByte & mask) == 0 {
		mask >>= 1
		length++
	}

	val := uint64(firstByte &^ mask)
	if length > 1 {
		buf := make([]byte, length-1)
		if _, err := io.ReadFull(d.file, buf); err != nil {
			return 0, 0, err
		}
		for _, v := range buf {
			val = (val << 8) | uint64(v)
		}
	}

	return val, length, nil
}

// Probe parses the EBML headers and maps track samples.
func (d *WebMDemuxer) Probe() ([]core.Track, error) {
	info, err := d.file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	trackMap := make(map[uint64]*core.Track)
	var tracks []core.Track
	var currentClusterTimecode int64
	globalMetadata := make(map[string]string)
	var globalChapters []core.Chapter

	offset := int64(0)
	for offset < fileSize {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}

		id, size, headerLen, err := d.ReadElementHeader()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		elementPayloadStart := offset + int64(headerLen)
		nextElementOffset := elementPayloadStart + size
		if size < 0 {
			nextElementOffset = elementPayloadStart
		}

		switch id {
		case 0x18538067: // Segment
			offset = elementPayloadStart
			continue
		case 0x1254C367: // Tags
			mMap, err := d.parseTags(elementPayloadStart, size)
			if err == nil {
				for k, v := range mMap {
					globalMetadata[k] = v
				}
			}
		case 0x1043A770: // Chapters
			chList, err := d.parseChapters(elementPayloadStart, size)
			if err == nil {
				globalChapters = chList
			}
		case 0x1549A966: // Info
			if err := d.parseInfo(elementPayloadStart, size); err != nil {
				return nil, err
			}
		case 0x1654AE6B: // Tracks
			tList, err := d.parseTracks(elementPayloadStart, size)
			if err != nil {
				return nil, err
			}
			tracks = tList
			for i := range tracks {
				trackMap[uint64(tracks[i].ID)] = &tracks[i]
			}
		case 0x1F43B675: // Cluster
			clusterOffset := elementPayloadStart
			clusterEnd := elementPayloadStart + size
			for clusterOffset < clusterEnd {
				if _, err := d.file.Seek(clusterOffset, io.SeekStart); err != nil {
					break
				}
				cid, csize, cheaderLen, cerr := d.ReadElementHeader()
				if cerr != nil {
					break
				}
				cPayloadStart := clusterOffset + int64(cheaderLen)
				switch cid {
				case 0xE7: // Timecode
					tBytes := make([]byte, csize)
					if _, err := io.ReadFull(d.file, tBytes); err == nil {
						var tc uint64
						for _, b := range tBytes {
							tc = (tc << 8) | uint64(b)
						}
						currentClusterTimecode = int64(tc)
					}
				case 0xA3: // SimpleBlock
					if _, err := d.file.Seek(cPayloadStart, io.SeekStart); err == nil {
						trackNum, tnLen, tnErr := d.readVINT()
						if tnErr == nil {
							var timecodeBytes [2]byte
							if _, tcErr := io.ReadFull(d.file, timecodeBytes[:]); tcErr == nil {
								relTimecode := int16(binary.BigEndian.Uint16(timecodeBytes[:]))
								var flags [1]byte
								_, _ = io.ReadFull(d.file, flags[:])
								isKey := (flags[0] & 0x80) != 0

								absTimecode := currentClusterTimecode + int64(relTimecode)
								payloadOffset := cPayloadStart + int64(tnLen) + 3
								payloadSize := csize - (int64(tnLen) + 3)

								tObj, exists := trackMap[trackNum]
								if exists {
									sample := core.Sample{
										ID:         len(tObj.Samples) + 1,
										IsKeyframe: isKey,
										Offset:     payloadOffset,
										Size:       payloadSize,
										Time:       absTimecode,
										Duration:   0,
									}
									if len(tObj.Samples) > 0 {
										prevIdx := len(tObj.Samples) - 1
										tObj.Samples[prevIdx].Duration = absTimecode - tObj.Samples[prevIdx].Time
									}
									tObj.Samples = append(tObj.Samples, sample)
								}
							}
						}
					}
				}
				clusterOffset = cPayloadStart + csize
			}
		}

		offset = nextElementOffset
	}

	for i := range tracks {
		t := &tracks[i]
		if len(t.Samples) > 0 {
			if len(t.Samples) > 1 {
				t.Samples[len(t.Samples)-1].Duration = t.Samples[len(t.Samples)-2].Duration
			} else {
				t.Samples[len(t.Samples)-1].Duration = 33 // Default to ~30 FPS (33ms)
			}
			t.Duration = uint64(t.Samples[len(t.Samples)-1].Time + t.Samples[len(t.Samples)-1].Duration)
		}
	}

	for i := range tracks {
		if len(globalMetadata) > 0 {
			tracks[i].Metadata = globalMetadata
		}
		if len(globalChapters) > 0 {
			tracks[i].Chapters = globalChapters
		}
	}

	d.tracks = tracks
	d.interleaved = d.buildInterleavedSamples()
	d.currentSample = 0
	return tracks, nil
}

func (d *WebMDemuxer) parseInfo(start int64, size int64) error {
	end := start + size
	offset := start
	for offset < end {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}
		id, csize, headerLen, err := d.ReadElementHeader()
		if err != nil {
			break
		}
		payloadStart := offset + int64(headerLen)
		if id == 0x2AD7B1 { // TimecodeScale
			tBytes := make([]byte, csize)
			if _, err := io.ReadFull(d.file, tBytes); err == nil {
				var scale uint64
				for _, b := range tBytes {
					scale = (scale << 8) | uint64(b)
				}
				d.timecodeScale = int64(scale)
			}
		}
		offset = payloadStart + csize
	}
	return nil
}

func (d *WebMDemuxer) parseTracks(start int64, size int64) ([]core.Track, error) {
	end := start + size
	offset := start
	var tracks []core.Track

	for offset < end {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}
		id, csize, headerLen, err := d.ReadElementHeader()
		if err != nil {
			break
		}
		payloadStart := offset + int64(headerLen)

		if id == 0xAE { // TrackEntry
			tr, err := d.parseTrackEntry(payloadStart, csize)
			if err == nil {
				tracks = append(tracks, *tr)
			}
		}

		offset = payloadStart + csize
	}
	return tracks, nil
}

func (d *WebMDemuxer) parseTrackEntry(start int64, size int64) (*core.Track, error) {
	end := start + size
	offset := start
	tr := &core.Track{
		Timescale: uint32(1000000000 / d.timecodeScale), // Timescale units per second
	}

	for offset < end {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}
		id, csize, headerLen, err := d.ReadElementHeader()
		if err != nil {
			break
		}
		payloadStart := offset + int64(headerLen)

		switch id {
		case 0xD7: // TrackNumber
			tBytes := make([]byte, csize)
			if _, err := io.ReadFull(d.file, tBytes); err == nil {
				var num uint64
				for _, b := range tBytes {
					num = (num << 8) | uint64(b)
				}
				tr.ID = int(num)
			}
		case 0x83: // TrackType
			tBytes := make([]byte, csize)
			if _, err := io.ReadFull(d.file, tBytes); err == nil {
				var tType uint64
				for _, b := range tBytes {
					tType = (tType << 8) | uint64(b)
				}
				if tType == 1 {
					tr.Type = core.TrackTypeVideo
				} else if tType == 2 {
					tr.Type = core.TrackTypeAudio
				} else {
					tr.Type = core.TrackTypeMeta
				}
			}
		case 0x86: // CodecID
			codecIDBytes := make([]byte, csize)
			if _, err := io.ReadFull(d.file, codecIDBytes); err == nil {
				tr.CodecTag = string(codecIDBytes)
			}
		case 0xE0: // Video
			videoEnd := payloadStart + csize
			videoOffset := payloadStart
			for videoOffset < videoEnd {
				if _, err := d.file.Seek(videoOffset, io.SeekStart); err != nil {
					break
				}
				vid, vsize, vheaderLen, verr := d.ReadElementHeader()
				if verr != nil {
					break
				}
				vpayloadStart := videoOffset + int64(vheaderLen)
				switch vid {
				case 0xB0: // PixelWidth
					wBytes := make([]byte, vsize)
					if _, err := io.ReadFull(d.file, wBytes); err == nil {
						var w uint32
						for _, b := range wBytes {
							w = (w << 8) | uint32(b)
						}
						tr.Width = w
					}
				case 0xBA: // PixelHeight
					hBytes := make([]byte, vsize)
					if _, err := io.ReadFull(d.file, hBytes); err == nil {
						var h uint32
						for _, b := range hBytes {
							h = (h << 8) | uint32(b)
						}
						tr.Height = h
					}
				case 0x7670: // Projection (holds rotation/pose roll)
					projEnd := vpayloadStart + vsize
					projOffset := vpayloadStart
					for projOffset < projEnd {
						if _, err := d.file.Seek(projOffset, io.SeekStart); err != nil {
							break
						}
						pid, psize, pheaderLen, perr := d.ReadElementHeader()
						if perr != nil {
							break
						}
						ppayloadStart := projOffset + int64(pheaderLen)
						if pid == 0x7675 { // ProjectionPoseRoll (float32 or float64)
							fBytes := make([]byte, psize)
							if _, err := io.ReadFull(d.file, fBytes); err == nil {
								var roll float64
								if psize == 4 {
									bits := binary.BigEndian.Uint32(fBytes)
									roll = float64(math.Float32frombits(bits))
								} else if psize == 8 {
									bits := binary.BigEndian.Uint64(fBytes)
									roll = math.Float64frombits(bits)
								}
								if tr.Metadata == nil {
									tr.Metadata = make(map[string]string)
								}
								tr.Metadata["rotation"] = fmt.Sprintf("%.2f", roll)
							}
						}
						projOffset = ppayloadStart + psize
					}
				}
				videoOffset = vpayloadStart + vsize
			}
		}

		offset = payloadStart + csize
	}

	return tr, nil
}

func (d *WebMDemuxer) buildInterleavedSamples() []core.InterleavedSample {
	var all []core.InterleavedSample
	for ti, t := range d.tracks {
		ts := float64(t.Timescale)
		if ts == 0 {
			ts = 1000
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

// ReadPacket fetches the next interleaved packet.
func (d *WebMDemuxer) ReadPacket() (*core.Packet, error) {
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
func (d *WebMDemuxer) Close() error {
	return d.file.Close()
}

func (d *WebMDemuxer) parseTags(startOffset int64, size int64) (map[string]string, error) {
	metaMap := make(map[string]string)
	endOffset := startOffset + size
	offset := startOffset

	for offset < endOffset {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}
		id, csize, headerLen, err := d.ReadElementHeader()
		if err != nil {
			break
		}
		payloadStart := offset + int64(headerLen)

		if id == 0x7373 { // Tag
			tagEnd := payloadStart + csize
			tagOffset := payloadStart
			for tagOffset < tagEnd {
				if _, err := d.file.Seek(tagOffset, io.SeekStart); err != nil {
					break
				}
				tid, tsize, theaderLen, terr := d.ReadElementHeader()
				if terr != nil {
					break
				}
				tPayloadStart := tagOffset + int64(theaderLen)

				if tid == 0x67C8 { // SimpleTag
					stEnd := tPayloadStart + tsize
					stOffset := tPayloadStart
					var tagName string
					var tagString string
					for stOffset < stEnd {
						if _, err := d.file.Seek(stOffset, io.SeekStart); err != nil {
							break
						}
						sid, ssize, sheaderLen, serr := d.ReadElementHeader()
						if serr != nil {
							break
						}
						sPayloadStart := stOffset + int64(sheaderLen)

						if sid == 0x45A3 { // TagName
							buf := make([]byte, ssize)
							if _, err := io.ReadFull(d.file, buf); err == nil {
								tagName = string(buf)
							}
						} else if sid == 0x4487 { // TagString
							buf := make([]byte, ssize)
							if _, err := io.ReadFull(d.file, buf); err == nil {
								tagString = string(buf)
							}
						}
						stOffset = sPayloadStart + ssize
					}

					if tagName != "" && tagString != "" {
						key := strings.ToLower(tagName)
						metaMap[key] = tagString
					}
				}
				tagOffset = tPayloadStart + tsize
			}
		}
		offset = payloadStart + csize
	}
	return metaMap, nil
}

func (d *WebMDemuxer) parseChapters(startOffset int64, size int64) ([]core.Chapter, error) {
	var chapters []core.Chapter
	endOffset := startOffset + size
	offset := startOffset

	for offset < endOffset {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}
		id, csize, headerLen, err := d.ReadElementHeader()
		if err != nil {
			break
		}
		payloadStart := offset + int64(headerLen)

		if id == 0x45B9 { // EditionEntry
			eeEnd := payloadStart + csize
			eeOffset := payloadStart
			for eeOffset < eeEnd {
				if _, err := d.file.Seek(eeOffset, io.SeekStart); err != nil {
					break
				}
				eid, esize, eheaderLen, eerr := d.ReadElementHeader()
				if eerr != nil {
					break
				}
				ePayloadStart := eeOffset + int64(eheaderLen)

				if eid == 0xB6 { // ChapterAtom
					caEnd := ePayloadStart + esize
					caOffset := ePayloadStart
					var startTimeNs uint64
					var title string
					for caOffset < caEnd {
						if _, err := d.file.Seek(caOffset, io.SeekStart); err != nil {
							break
						}
						cid, csize, cheaderLen, cerr := d.ReadElementHeader()
						if cerr != nil {
							break
						}
						cPayloadStart := caOffset + int64(cheaderLen)

						if cid == 0x91 { // ChapterTimeStart
							tBytes := make([]byte, csize)
							if _, err := io.ReadFull(d.file, tBytes); err == nil {
								var ts uint64
								for _, b := range tBytes {
									ts = (ts << 8) | uint64(b)
								}
								startTimeNs = ts
							}
						} else if cid == 0x80 { // ChapterDisplay
							cdEnd := cPayloadStart + csize
							cdOffset := cPayloadStart
							for cdOffset < cdEnd {
								if _, err := d.file.Seek(cdOffset, io.SeekStart); err != nil {
									break
								}
								cdid, cdsize, cdheaderLen, cderr := d.ReadElementHeader()
								if cderr != nil {
									break
								}
								cdPayloadStart := cdOffset + int64(cdheaderLen)

								if cdid == 0x85 { // ChapString
									buf := make([]byte, cdsize)
									if _, err := io.ReadFull(d.file, buf); err == nil {
										title = string(buf)
									}
								}
								cdOffset = cdPayloadStart + cdsize
							}
						}
						caOffset = cPayloadStart + csize
					}

					startTime := int64(startTimeNs / 1000000)

					chapters = append(chapters, core.Chapter{
						StartTime: startTime,
						Title:     title,
					})
				}
				eeOffset = ePayloadStart + esize
			}
		}
		offset = payloadStart + csize
	}
	return chapters, nil
}
