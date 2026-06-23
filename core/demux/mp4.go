package demux

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"cromedia/core"
)

// MP4Demuxer handles demuxing of MP4 files.
type MP4Demuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
}

// NewMP4Demuxer instantiates a new MP4 demuxer.
func NewMP4Demuxer(file *os.File) *MP4Demuxer {
	return &MP4Demuxer{file: file}
}

// Probe parses headers and initializes track metadata.
func (d *MP4Demuxer) Probe() ([]core.Track, error) {
	atoms, err := core.FastProbe(d.file)
	if err != nil {
		return nil, err
	}

	var findAtom func(atoms []core.Atom, typ string) *core.Atom
	findAtom = func(atoms []core.Atom, typ string) *core.Atom {
		for i := range atoms {
			if atoms[i].Type == typ {
				return &atoms[i]
			}
		}
		return nil
	}

	moov := findAtom(atoms, "moov")
	if moov == nil {
		return nil, fmt.Errorf("error: 'moov' atom not found")
	}

	tracks, err := d.ExtractTracks(*moov)
	if err != nil {
		return nil, err
	}
	d.tracks = tracks

	// Parse any movie fragments (moof) at top level
	for _, atom := range atoms {
		if atom.Type == "moof" {
			err = d.parseMoof(atom)
			if err != nil {
				return nil, err
			}
		}
	}

	d.interleaved = d.buildInterleavedSamples()
	d.currentSample = 0
	return tracks, nil
}

// ReadPacket retrieves the next available packet in interleaved order.
func (d *MP4Demuxer) ReadPacket() (*core.Packet, error) {
	if d.currentSample >= len(d.interleaved) {
		return nil, io.EOF
	}

	is := d.interleaved[d.currentSample]
	d.currentSample++

	// Seek to sample offset
	_, err := d.file.Seek(is.Sample.Offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	// Allocate buffer using BufferPool
	buf := core.GlobalGet(int(is.Sample.Size))
	_, err = io.ReadFull(d.file, buf)
	if err != nil {
		core.GlobalPut(buf)
		return nil, err
	}

	pkt := &core.Packet{
		StreamIndex: is.TrackIndex,
		Data:        buf,
		PTS:         is.Sample.Time,
		DTS:         is.Sample.Time,
		Duration:    is.Sample.Duration,
		IsKeyframe:  is.Sample.IsKeyframe,
	}

	// If B-Frame CTS offset exists, apply it to PTS
	track := d.tracks[is.TrackIndex]
	if len(track.CTSOffsets) > is.SampleIndex {
		pkt.PTS = is.Sample.Time + int64(track.CTSOffsets[is.SampleIndex])
	}

	return pkt, nil
}

// Close releases resources.
func (d *MP4Demuxer) Close() error {
	return d.file.Close()
}

func (d *MP4Demuxer) buildInterleavedSamples() []core.InterleavedSample {
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

// Helper to find child by type
func findChildPath(parent core.Atom, typ string) *core.Atom {
	for _, c := range parent.Children {
		if c.Type == typ {
			return &c
		}
	}
	return nil
}

// Helper to read payload
func readPayload(f *os.File, atom *core.Atom) []byte {
	if _, err := f.Seek(atom.Offset+8, 0); err != nil {
		return nil
	}
	buf := make([]byte, atom.Size-8)
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil
	}
	return buf
}

// ExtractTracks parses all tracks from the Movie Atom
func (d *MP4Demuxer) ExtractTracks(moov core.Atom) ([]core.Track, error) {
	var tracks []core.Track

	metaMap := d.parseMetadata(moov)
	chapters := d.parseChapters(moov)

	for _, child := range moov.Children {
		if child.Type == "trak" {
			track, err := d.parseTrack(child)
			if err != nil {
				fmt.Printf("[Demuxer] Warning: Failed to parse track: %v\n", err)
				continue
			}
			track.Metadata = metaMap
			track.Chapters = chapters
			tracks = append(tracks, *track)
		}
	}

	if len(tracks) == 0 {
		return nil, fmt.Errorf("no valid tracks found in moov")
	}

	return tracks, nil
}

func (d *MP4Demuxer) parseMetadata(moov core.Atom) map[string]string {
	metaMap := make(map[string]string)
	udta := findChildPath(moov, "udta")
	if udta == nil {
		return metaMap
	}
	meta := findChildPath(*udta, "meta")
	if meta == nil {
		return metaMap
	}
	ilst := findChildPath(*meta, "ilst")
	if ilst == nil {
		return metaMap
	}

	for _, item := range ilst.Children {
		dataBox := findChildPath(item, "data")
		if dataBox == nil {
			continue
		}
		payload := readPayload(d.file, dataBox)
		if len(payload) <= 8 {
			continue
		}
		// Skip 4 bytes version/flags + 4 bytes class
		value := string(payload[8:])
		key := item.Type
		switch key {
		case "\xa9nam":
			metaMap["title"] = value
		case "\xa9ART":
			metaMap["artist"] = value
		case "\xa9alb":
			metaMap["album"] = value
		case "\xa9day":
			metaMap["year"] = value
		case "\xa9too":
			metaMap["tool"] = value
		default:
			if strings.HasPrefix(key, "\xa9") {
				metaMap[strings.TrimPrefix(key, "\xa9")] = value
			} else {
				metaMap[key] = value
			}
		}
	}
	return metaMap
}

func (d *MP4Demuxer) parseChapters(moov core.Atom) []core.Chapter {
	var chapters []core.Chapter
	udta := findChildPath(moov, "udta")
	if udta == nil {
		return chapters
	}
	chpl := findChildPath(*udta, "chpl")
	if chpl == nil {
		return chapters
	}

	payload := readPayload(d.file, chpl)
	if len(payload) <= 8 {
		return chapters
	}

	version := payload[0]
	var numChapters uint32
	var idx = 4

	if version == 1 {
		idx = 9
		if len(payload) < idx+4 {
			return chapters
		}
		numChapters = binary.BigEndian.Uint32(payload[idx-4 : idx])
	} else {
		idx = 5
		if len(payload) < idx+4 {
			return chapters
		}
		numChapters = binary.BigEndian.Uint32(payload[idx-1 : idx+3])
		idx += 3
	}

	for i := uint32(0); i < numChapters; i++ {
		if len(payload) < idx+9 {
			break
		}
		startTime := int64(binary.BigEndian.Uint64(payload[idx : idx+8]))
		titleLen := int(payload[idx+8])
		idx += 9

		if len(payload) < idx+titleLen {
			break
		}
		title := string(payload[idx : idx+titleLen])
		idx += titleLen

		chapters = append(chapters, core.Chapter{
			StartTime: startTime,
			Title:     title,
		})
	}

	return chapters
}

// parseTrack parses a single 'trak' atom into a Track struct
func (d *MP4Demuxer) parseTrack(trak core.Atom) (*core.Track, error) {
	tr := &core.Track{}

	// 1. tkhd (Track Header)
	tkhdAtom := findChildPath(trak, "tkhd")
	if tkhdAtom == nil {
		return nil, fmt.Errorf("missing tkhd")
	}
	tr.Tkhd = readPayload(d.file, tkhdAtom)
	// Parse Track ID, Width, Height, and Matrix (Best effort)
	trackID, width, height, matrix, _ := d.ParseTkhd(*tkhdAtom)
	tr.ID = int(trackID)
	tr.Width = width
	tr.Height = height
	tr.Matrix = matrix

	// 1b. edts -> elst (Edit List) — Sync correction
	edtsAtom := findChildPath(trak, "edts")
	if edtsAtom != nil {
		elstAtom := findChildPath(*edtsAtom, "elst")
		if elstAtom != nil {
			entries, parseErr := d.ParseElst(*elstAtom)
			if parseErr == nil {
				tr.EditList = entries
				// Compute MediaTimeOffset from first non-empty edit
				for _, e := range entries {
					if e.MediaTime >= 0 {
						tr.MediaTimeOffset = e.MediaTime
						break
					}
				}
				fmt.Printf("[Demuxer] Track edts: %d edit list entries, MediaTimeOffset=%d\n", len(entries), tr.MediaTimeOffset)
			}
		}
	}

	// 2. mdia -> mdhd (Media Header - Timescale)
	mdiaAtom := findChildPath(trak, "mdia")
	if mdiaAtom == nil {
		return nil, fmt.Errorf("missing mdia")
	}
	mdhdAtom := findChildPath(*mdiaAtom, "mdhd")
	if mdhdAtom == nil {
		return nil, fmt.Errorf("missing mdhd")
	}
	timescale, duration, err := d.ParseMdhd(*mdhdAtom)
	if err != nil {
		return nil, err
	}
	tr.Timescale = timescale
	tr.Duration = duration

	// 3. mdia -> hdlr (Handler - Type)
	hdlrAtom := findChildPath(*mdiaAtom, "hdlr")
	if hdlrAtom == nil {
		return nil, fmt.Errorf("missing hdlr")
	}
	tr.Hdlr = readPayload(d.file, hdlrAtom)

	// Determine Type from hdlr
	if len(tr.Hdlr) >= 12 {
		handlerType := string(tr.Hdlr[8:12]) // Offset 8 (after Ver/Flags/Pre)
		switch handlerType {
		case "vide":
			tr.Type = core.TrackTypeVideo
		case "soun":
			tr.Type = core.TrackTypeAudio
		case "hint":
			tr.Type = core.TrackTypeHint
		default:
			tr.Type = core.TrackTypeMeta
		}
	}

	// 4. mdia -> minf (Media Info)
	minfAtom := findChildPath(*mdiaAtom, "minf")
	if minfAtom == nil {
		return nil, fmt.Errorf("missing minf")
	}

	// Media Header (vmhd or smhd)
	if tr.Type == core.TrackTypeVideo {
		vmhdAtom := findChildPath(*minfAtom, "vmhd")
		if vmhdAtom != nil {
			tr.MediaHeader = readPayload(d.file, vmhdAtom)
		}
	} else if tr.Type == core.TrackTypeAudio {
		smhdAtom := findChildPath(*minfAtom, "smhd")
		if smhdAtom != nil {
			tr.MediaHeader = readPayload(d.file, smhdAtom)
		}
	}

	// 5. stbl (Sample Table) - The Big One
	samples, err := d.MapSamples(trak)
	if err != nil {
		return nil, fmt.Errorf("failed to map samples: %v", err)
	}
	tr.Samples = samples

	// 6. stsd (Sample Description) - for Codec Config
	stblAtom := findChildPath(*minfAtom, "stbl")
	if stblAtom != nil {
		stsdAtom := findChildPath(*stblAtom, "stsd")
		if stsdAtom != nil {
			tr.Stsd = readPayload(d.file, stsdAtom)
		}

		// 7. ctts (Composition Time to Sample) - B-Frame support
		cttsAtom := findChildPath(*stblAtom, "ctts")
		if cttsAtom != nil {
			ctsEntries, parseErr := d.ParseCtts(*cttsAtom)
			if parseErr == nil {
				// Expand CTTS entries into per-sample offsets
				var offsets []int32
				for _, e := range ctsEntries {
					for j := 0; j < int(e.Count); j++ {
						offsets = append(offsets, e.Offset)
					}
				}
				tr.CTSOffsets = offsets
				fmt.Printf("[Demuxer] Track %s: Loaded %d ctts entries (%d per-sample offsets)\n", tr.Type, len(ctsEntries), len(offsets))
			}
		}
	}

	// 8. Codec Detection from stsd payload
	if len(tr.Stsd) >= 12 {
		tr.CodecTag = string(tr.Stsd[12:16])
		fmt.Printf("[Demuxer] Track %s: Codec Tag = '%s'\n", tr.Type, tr.CodecTag)
		d.parseCodecPrivateAndHDR(tr)
	}

	return tr, nil
}

func (d *MP4Demuxer) parseCodecPrivateAndHDR(tr *core.Track) {
	if len(tr.Stsd) < 16 {
		return
	}
	if tr.Metadata == nil {
		tr.Metadata = make(map[string]string)
	}

	entryCount := binary.BigEndian.Uint32(tr.Stsd[4:8])
	if entryCount == 0 {
		return
	}

	idx := 8
	if len(tr.Stsd) < idx+8 {
		return
	}
	entrySize := int(binary.BigEndian.Uint32(tr.Stsd[idx : idx+4]))
	if len(tr.Stsd) < idx+entrySize {
		return
	}

	if tr.Type == core.TrackTypeVideo {
		subBoxesStart := idx + 8 + 78
		subBoxesEnd := idx + entrySize

		currIdx := subBoxesStart
		for currIdx+8 <= subBoxesEnd && currIdx < len(tr.Stsd) {
			boxSize := int(binary.BigEndian.Uint32(tr.Stsd[currIdx : currIdx+4]))
			boxType := string(tr.Stsd[currIdx+4 : currIdx+8])

			if boxSize <= 0 || currIdx+boxSize > subBoxesEnd {
				break
			}

			payloadOffset := currIdx + 8
			payloadSize := boxSize - 8

			switch boxType {
			case "colr":
				if payloadSize >= 11 {
					colorType := string(tr.Stsd[payloadOffset : payloadOffset+4])
					if colorType == "nclx" {
						primaries := binary.BigEndian.Uint16(tr.Stsd[payloadOffset+4 : payloadOffset+6])
						transfer := binary.BigEndian.Uint16(tr.Stsd[payloadOffset+6 : payloadOffset+8])
						matrix := binary.BigEndian.Uint16(tr.Stsd[payloadOffset+8 : payloadOffset+10])
						fullRange := tr.Stsd[payloadOffset+10]

						tr.Metadata["color_primaries"] = fmt.Sprintf("%d", primaries)
						tr.Metadata["color_transfer"] = fmt.Sprintf("%d", transfer)
						tr.Metadata["color_matrix"] = fmt.Sprintf("%d", matrix)
						tr.Metadata["color_full_range"] = fmt.Sprintf("%d", fullRange)
					}
				}
			case "clli":
				if payloadSize >= 4 {
					maxCLL := binary.BigEndian.Uint16(tr.Stsd[payloadOffset : payloadOffset+2])
					maxFALL := binary.BigEndian.Uint16(tr.Stsd[payloadOffset+2 : payloadOffset+4])
					tr.Metadata["max_cll"] = fmt.Sprintf("%d", maxCLL)
					tr.Metadata["max_fall"] = fmt.Sprintf("%d", maxFALL)
				}
			case "mdcv":
				if payloadSize >= 24 {
					tr.Metadata["hdr_enabled"] = "true"
					tr.Metadata["mastering_display"] = fmt.Sprintf("%x", tr.Stsd[payloadOffset:payloadOffset+24])
				}
			case "avcC", "hvcC", "av1C":
				tr.CodecPrivate = append([]byte{}, tr.Stsd[currIdx:currIdx+boxSize]...)
			}

			currIdx += boxSize
		}
	}
}

// Helper to read FullBox header (Version + Flags)
func readFullBoxHeader(r io.Reader) (version uint8, flags uint32, err error) {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, 0, err
	}
	val := binary.BigEndian.Uint32(buf)
	version = uint8(val >> 24)
	flags = val & 0x00FFFFFF
	return
}

// ParseStts parses Time-to-Sample box
func (d *MP4Demuxer) ParseStts(atom core.Atom) ([]struct{ Count, Duration uint32 }, error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return nil, err
	}
	_, _, err := readFullBoxHeader(d.file)
	if err != nil {
		return nil, err
	}

	var entryCount uint32
	if err := binary.Read(d.file, binary.BigEndian, &entryCount); err != nil {
		return nil, err
	}

	entries := make([]struct{ Count, Duration uint32 }, entryCount)
	for i := 0; i < int(entryCount); i++ {
		if err := binary.Read(d.file, binary.BigEndian, &entries[i]); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

// ParseStss parses Sync Sample box (Keyframes)
func (d *MP4Demuxer) ParseStss(atom core.Atom) ([]uint32, error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return nil, err
	}
	_, _, err := readFullBoxHeader(d.file)
	if err != nil {
		return nil, err
	}

	var entryCount uint32
	if err := binary.Read(d.file, binary.BigEndian, &entryCount); err != nil {
		return nil, err
	}

	entries := make([]uint32, entryCount)
	for i := 0; i < int(entryCount); i++ {
		if err := binary.Read(d.file, binary.BigEndian, &entries[i]); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

// ParseStco parses Chunk Offset box
func (d *MP4Demuxer) ParseStco(atom core.Atom) ([]uint32, error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return nil, err
	}
	_, _, err := readFullBoxHeader(d.file)
	if err != nil {
		return nil, err
	}

	var entryCount uint32
	if err := binary.Read(d.file, binary.BigEndian, &entryCount); err != nil {
		return nil, err
	}

	entries := make([]uint32, entryCount)
	for i := 0; i < int(entryCount); i++ {
		if err := binary.Read(d.file, binary.BigEndian, &entries[i]); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

// ParseStsz parses Sample Size box
func (d *MP4Demuxer) ParseStsz(atom core.Atom) (uint32, []uint32, error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return 0, nil, err
	}
	_, _, err := readFullBoxHeader(d.file)
	if err != nil {
		return 0, nil, err
	}

	var sampleSize uint32
	if err := binary.Read(d.file, binary.BigEndian, &sampleSize); err != nil {
		return 0, nil, err
	}

	var entryCount uint32
	if err := binary.Read(d.file, binary.BigEndian, &entryCount); err != nil {
		return 0, nil, err
	}

	if sampleSize != 0 {
		return sampleSize, nil, nil
	}

	entries := make([]uint32, entryCount)
	for i := 0; i < int(entryCount); i++ {
		if err := binary.Read(d.file, binary.BigEndian, &entries[i]); err != nil {
			return 0, nil, err
		}
	}
	return 0, entries, nil
}

// ParseStsc parses Sample-to-Chunk box
func (d *MP4Demuxer) ParseStsc(atom core.Atom) ([]struct{ FirstChunk, SamplesPerChunk, SampleDescID uint32 }, error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return nil, err
	}
	_, _, err := readFullBoxHeader(d.file)
	if err != nil {
		return nil, err
	}

	var entryCount uint32
	if err := binary.Read(d.file, binary.BigEndian, &entryCount); err != nil {
		return nil, err
	}

	entries := make([]struct{ FirstChunk, SamplesPerChunk, SampleDescID uint32 }, entryCount)
	for i := 0; i < int(entryCount); i++ {
		if err := binary.Read(d.file, binary.BigEndian, &entries[i]); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

// ParseCtts parses Composition Time to Sample box (B-Frame ordering)
func (d *MP4Demuxer) ParseCtts(atom core.Atom) ([]struct {
	Count  uint32
	Offset int32
}, error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return nil, err
	}
	version, _, err := readFullBoxHeader(d.file)
	if err != nil {
		return nil, err
	}

	var entryCount uint32
	if err := binary.Read(d.file, binary.BigEndian, &entryCount); err != nil {
		return nil, err
	}

	entries := make([]struct {
		Count  uint32
		Offset int32
	}, entryCount)
	for i := 0; i < int(entryCount); i++ {
		if err := binary.Read(d.file, binary.BigEndian, &entries[i].Count); err != nil {
			return nil, err
		}
		if version == 0 {
			var uoff uint32
			if err := binary.Read(d.file, binary.BigEndian, &uoff); err != nil {
				return nil, err
			}
			entries[i].Offset = int32(uoff)
		} else {
			if err := binary.Read(d.file, binary.BigEndian, &entries[i].Offset); err != nil {
				return nil, err
			}
		}
	}
	return entries, nil
}

// ParseElst parses Edit List box for A/V sync correction
func (d *MP4Demuxer) ParseElst(atom core.Atom) ([]core.EditListEntry, error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return nil, err
	}
	version, _, err := readFullBoxHeader(d.file)
	if err != nil {
		return nil, err
	}

	var entryCount uint32
	if err := binary.Read(d.file, binary.BigEndian, &entryCount); err != nil {
		return nil, err
	}

	entries := make([]core.EditListEntry, entryCount)
	for i := 0; i < int(entryCount); i++ {
		if version == 1 {
			var segDur uint64
			var mediaTime int64
			if err := binary.Read(d.file, binary.BigEndian, &segDur); err != nil {
				return nil, err
			}
			if err := binary.Read(d.file, binary.BigEndian, &mediaTime); err != nil {
				return nil, err
			}
			entries[i].SegmentDuration = segDur
			entries[i].MediaTime = mediaTime
		} else {
			var segDur32 uint32
			var mediaTime32 int32
			if err := binary.Read(d.file, binary.BigEndian, &segDur32); err != nil {
				return nil, err
			}
			if err := binary.Read(d.file, binary.BigEndian, &mediaTime32); err != nil {
				return nil, err
			}
			entries[i].SegmentDuration = uint64(segDur32)
			entries[i].MediaTime = int64(mediaTime32)
		}
		if err := binary.Read(d.file, binary.BigEndian, &entries[i].MediaRateInt); err != nil {
			return nil, err
		}
		if err := binary.Read(d.file, binary.BigEndian, &entries[i].MediaRateFrac); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

// ParseMdhd parses Media Header to get Timescale
func (d *MP4Demuxer) ParseMdhd(atom core.Atom) (uint32, uint64, error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return 0, 0, err
	}
	version, _, err := readFullBoxHeader(d.file)
	if err != nil {
		return 0, 0, err
	}

	var timescale uint32
	var duration uint64

	if version == 1 {
		if _, err := d.file.Seek(16, io.SeekCurrent); err != nil {
			return 0, 0, err
		}
		if err := binary.Read(d.file, binary.BigEndian, &timescale); err != nil {
			return 0, 0, err
		}
		if err := binary.Read(d.file, binary.BigEndian, &duration); err != nil {
			return 0, 0, err
		}
	} else {
		if _, err := d.file.Seek(8, io.SeekCurrent); err != nil {
			return 0, 0, err
		}
		if err := binary.Read(d.file, binary.BigEndian, &timescale); err != nil {
			return 0, 0, err
		}
		var dur32 uint32
		if err := binary.Read(d.file, binary.BigEndian, &dur32); err != nil {
			return 0, 0, err
		}
		duration = uint64(dur32)
	}

	return timescale, duration, nil
}

// ParseTkhd parses Track Header to get Track ID, Width, Height, and Matrix
func (d *MP4Demuxer) ParseTkhd(atom core.Atom) (trackID uint32, width, height uint32, matrix []byte, err error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return 0, 0, 0, nil, err
	}
	version, _, err := readFullBoxHeader(d.file)
	if err != nil {
		return 0, 0, 0, nil, err
	}

	if version == 0 {
		if _, err := d.file.Seek(8, io.SeekCurrent); err != nil {
			return 0, 0, 0, nil, err
		}
	} else {
		if _, err := d.file.Seek(16, io.SeekCurrent); err != nil {
			return 0, 0, 0, nil, err
		}
	}

	if err := binary.Read(d.file, binary.BigEndian, &trackID); err != nil {
		return 0, 0, 0, nil, err
	}

	skipRest := int64(24)
	if version == 1 {
		skipRest = 28
	}
	if _, err := d.file.Seek(skipRest, io.SeekCurrent); err != nil {
		return 0, 0, 0, nil, err
	}

	matrix = make([]byte, 36)
	if _, err := io.ReadFull(d.file, matrix); err != nil {
		return 0, 0, 0, nil, err
	}

	if err := binary.Read(d.file, binary.BigEndian, &width); err != nil {
		return 0, 0, 0, nil, err
	}
	if err := binary.Read(d.file, binary.BigEndian, &height); err != nil {
		return 0, 0, 0, nil, err
	}

	return trackID, width, height, matrix, nil
}

// LocateTables finds the stbl children from a trak atom (scoped)
func (d *MP4Demuxer) LocateTables(moov core.Atom) (stss, stts, stco, stsz, stsc *core.Atom) {
	var find func(atoms []core.Atom)
	find = func(atoms []core.Atom) {
		for i := range atoms {
			switch atoms[i].Type {
			case "stss":
				stss = &atoms[i]
			case "stts":
				stts = &atoms[i]
			case "stco":
				stco = &atoms[i]
			case "stsz":
				stsz = &atoms[i]
			case "stsc":
				stsc = &atoms[i]
			default:
				if len(atoms[i].Children) > 0 {
					find(atoms[i].Children)
				}
			}
		}
	}
	find(moov.Children)
	return
}

// MapSamples processes all tables to generate a flat list of Samples with offsets and times
func (d *MP4Demuxer) MapSamples(moov core.Atom) ([]core.Sample, error) {
	stssAtom, sttsAtom, stcoAtom, stszAtom, stscAtom := d.LocateTables(moov)
	if sttsAtom == nil || stcoAtom == nil || stszAtom == nil || stscAtom == nil {
		return nil, fmt.Errorf("missing critical atom tables (stts, stco, stsz, or stsc)")
	}

	stts, err := d.ParseStts(*sttsAtom)
	if err != nil {
		return nil, err
	}

	stco, err := d.ParseStco(*stcoAtom)
	if err != nil {
		return nil, err
	}

	fixedSize, stsz, err := d.ParseStsz(*stszAtom)
	if err != nil {
		return nil, err
	}

	stsc, err := d.ParseStsc(*stscAtom)
	if err != nil {
		return nil, err
	}

	var stss []uint32
	if stssAtom != nil {
		stss, err = d.ParseStss(*stssAtom)
		if err != nil {
			return nil, err
		}
	}

	isKeyframe := make(map[int]bool)
	if len(stss) == 0 {
	} else {
		for _, id := range stss {
			isKeyframe[int(id)] = true
		}
	}

	numSamples := 0
	if fixedSize != 0 {
		numSamples = 0
		for _, entry := range stts {
			numSamples += int(entry.Count)
		}
	} else {
		numSamples = len(stsz)
	}

	samples := make([]core.Sample, numSamples)

	currentSample := 0
	currentTime := int64(0)
	for _, entry := range stts {
		for i := 0; i < int(entry.Count); i++ {
			if currentSample < len(samples) {
				samples[currentSample].Time = currentTime
				samples[currentSample].Duration = int64(entry.Duration)
				samples[currentSample].ID = currentSample + 1
				currentTime += int64(entry.Duration)
				currentSample++
			}
		}
	}

	if fixedSize != 0 {
		for i := range samples {
			samples[i].Size = int64(fixedSize)
		}
	} else {
		for i := 0; i < len(stsz) && i < len(samples); i++ {
			samples[i].Size = int64(stsz[i])
		}
	}

	sampleIdx := 0
	for i, chunkOffset := range stco {
		chunkIndex := i + 1

		var samplesPerChunk uint32
		for j := 0; j < len(stsc); j++ {
			if uint32(chunkIndex) >= stsc[j].FirstChunk {
				samplesPerChunk = stsc[j].SamplesPerChunk
			} else {
				break
			}
		}

		offset := int64(chunkOffset)
		for j := 0; j < int(samplesPerChunk); j++ {
			if sampleIdx < len(samples) {
				samples[sampleIdx].Offset = offset
				samples[sampleIdx].IsKeyframe = (len(stss) == 0) || isKeyframe[samples[sampleIdx].ID]

				offset += samples[sampleIdx].Size
				sampleIdx++
			}
		}
	}

	return samples, nil
}

type trunSample struct {
	Duration              uint32
	Size                  uint32
	Flags                 uint32
	CompositionTimeOffset int32
}

func (d *MP4Demuxer) parseMoof(moof core.Atom) error {
	for _, child := range moof.Children {
		if child.Type != "traf" {
			continue
		}

		var tfhd *core.Atom
		var tfdt *core.Atom
		var truns []*core.Atom

		for i := range child.Children {
			c := &child.Children[i]
			switch c.Type {
			case "tfhd":
				tfhd = c
			case "tfdt":
				tfdt = c
			case "trun":
				truns = append(truns, c)
			}
		}

		if tfhd == nil {
			return fmt.Errorf("missing tfhd in traf fragment")
		}

		trackID, defaultDuration, defaultSize, _, baseDataOffset, err := d.parseTfhd(*tfhd)
		if err != nil {
			return err
		}

		var track *core.Track
		for i := range d.tracks {
			if uint32(d.tracks[i].ID) == trackID {
				track = &d.tracks[i]
				break
			}
		}
		if track == nil {
			continue
		}

		baseMediaDecodeTime := int64(-1)
		if tfdt != nil {
			bmdt, err := d.parseTfdt(*tfdt)
			if err != nil {
				return err
			}
			baseMediaDecodeTime = bmdt
		}

		if baseMediaDecodeTime < 0 {
			if len(track.Samples) > 0 {
				last := track.Samples[len(track.Samples)-1]
				baseMediaDecodeTime = last.Time + last.Duration
			} else {
				baseMediaDecodeTime = 0
			}
		}

		if baseDataOffset == -1 {
			baseDataOffset = moof.Offset
		}

		currentTime := baseMediaDecodeTime
		sampleID := len(track.Samples) + 1

		for _, trunAtom := range truns {
			runSamples, dataOffset, err := d.parseTrun(*trunAtom)
			if err != nil {
				return err
			}

			currSampleOffset := baseDataOffset
			if dataOffset != 0 {
				currSampleOffset += int64(dataOffset)
			}

			for _, rs := range runSamples {
				dur := int64(rs.Duration)
				if dur == 0 {
					dur = int64(defaultDuration)
				}
				sz := int64(rs.Size)
				if sz == 0 {
					sz = int64(defaultSize)
				}
				isKey := (rs.Flags&0x00010000) == 0
				if (rs.Flags & 0x02000000) != 0 {
					isKey = false
				} else if len(runSamples) == 1 || rs.Flags == 0 {
					isKey = true
				}

				sample := core.Sample{
					ID:         sampleID,
					IsKeyframe: isKey,
					Offset:     currSampleOffset,
					Size:       sz,
					Time:       currentTime,
					Duration:   dur,
				}

				track.Samples = append(track.Samples, sample)
				for len(track.CTSOffsets) < len(track.Samples)-1 {
					track.CTSOffsets = append(track.CTSOffsets, 0)
				}
				track.CTSOffsets = append(track.CTSOffsets, rs.CompositionTimeOffset)

				currentTime += dur
				currSampleOffset += sz
				sampleID++
			}
		}

		if len(track.Samples) > 0 {
			last := track.Samples[len(track.Samples)-1]
			track.Duration = uint64(last.Time + last.Duration)
		}
	}
	return nil
}

func (d *MP4Demuxer) parseTfhd(atom core.Atom) (trackID uint32, defaultDuration, defaultSize, defaultFlags uint32, baseDataOffset int64, err error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return 0, 0, 0, 0, 0, err
	}
	_, flags, err := readFullBoxHeader(d.file)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	if err := binary.Read(d.file, binary.BigEndian, &trackID); err != nil {
		return 0, 0, 0, 0, 0, err
	}

	baseDataOffset = -1
	if (flags & 0x000001) != 0 {
		var bdo uint64
		if err := binary.Read(d.file, binary.BigEndian, &bdo); err != nil {
			return 0, 0, 0, 0, 0, err
		}
		baseDataOffset = int64(bdo)
	}

	if (flags & 0x000002) != 0 {
		var sdi uint32
		if err := binary.Read(d.file, binary.BigEndian, &sdi); err != nil {
			return 0, 0, 0, 0, 0, err
		}
	}

	if (flags & 0x000008) != 0 {
		if err := binary.Read(d.file, binary.BigEndian, &defaultDuration); err != nil {
			return 0, 0, 0, 0, 0, err
		}
	}

	if (flags & 0x000010) != 0 {
		if err := binary.Read(d.file, binary.BigEndian, &defaultSize); err != nil {
			return 0, 0, 0, 0, 0, err
		}
	}

	if (flags & 0x000020) != 0 {
		if err := binary.Read(d.file, binary.BigEndian, &defaultFlags); err != nil {
			return 0, 0, 0, 0, 0, err
		}
	}

	return
}

func (d *MP4Demuxer) parseTfdt(atom core.Atom) (baseMediaDecodeTime int64, err error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return 0, err
	}
	version, _, err := readFullBoxHeader(d.file)
	if err != nil {
		return 0, err
	}

	if version == 1 {
		var bmdt uint64
		if err := binary.Read(d.file, binary.BigEndian, &bmdt); err != nil {
			return 0, err
		}
		baseMediaDecodeTime = int64(bmdt)
	} else {
		var bmdt32 uint32
		if err := binary.Read(d.file, binary.BigEndian, &bmdt32); err != nil {
			return 0, err
		}
		baseMediaDecodeTime = int64(bmdt32)
	}
	return
}

func (d *MP4Demuxer) parseTrun(atom core.Atom) (samples []trunSample, dataOffset int32, err error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return nil, 0, err
	}
	version, flags, err := readFullBoxHeader(d.file)
	if err != nil {
		return nil, 0, err
	}

	var sampleCount uint32
	if err := binary.Read(d.file, binary.BigEndian, &sampleCount); err != nil {
		return nil, 0, err
	}

	if (flags & 0x000001) != 0 {
		if err := binary.Read(d.file, binary.BigEndian, &dataOffset); err != nil {
			return nil, 0, err
		}
	}

	var firstSampleFlags uint32
	if (flags & 0x000004) != 0 {
		if err := binary.Read(d.file, binary.BigEndian, &firstSampleFlags); err != nil {
			return nil, 0, err
		}
	}

	samples = make([]trunSample, sampleCount)
	for i := 0; i < int(sampleCount); i++ {
		s := trunSample{}
		if (flags & 0x000100) != 0 {
			if err := binary.Read(d.file, binary.BigEndian, &s.Duration); err != nil {
				return nil, 0, err
			}
		}
		if (flags & 0x000200) != 0 {
			if err := binary.Read(d.file, binary.BigEndian, &s.Size); err != nil {
				return nil, 0, err
			}
		}
		if (flags & 0x000400) != 0 {
			if err := binary.Read(d.file, binary.BigEndian, &s.Flags); err != nil {
				return nil, 0, err
			}
		} else {
			s.Flags = firstSampleFlags
		}
		if (flags & 0x000800) != 0 {
			if version == 0 {
				var uoff uint32
				if err := binary.Read(d.file, binary.BigEndian, &uoff); err != nil {
					return nil, 0, err
				}
				s.CompositionTimeOffset = int32(uoff)
			} else {
				if err := binary.Read(d.file, binary.BigEndian, &s.CompositionTimeOffset); err != nil {
					return nil, 0, err
				}
			}
		}
		samples[i] = s
	}

	return
}

