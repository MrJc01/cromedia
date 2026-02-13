package core

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Sample represents a single video frame/audio sample
type Sample struct {
	ID         int
	IsKeyframe bool
	Offset     int64
	Size       int64
	Time       int64 // Decoding time
	Duration   int64
}

// KeyframeInfo holds metadata for cutting
type KeyframeInfo struct {
	Timestamp int64 // Duration units (timescale)
	Offset    int64 // Byte offset in file
}

// Demuxer handles the parsing of the Sample Table (stbl)
type Demuxer struct {
	file *os.File
}

func NewDemuxer(file *os.File) *Demuxer {
	return &Demuxer{file: file}
}

// Helper to find child by type
func findChildPath(parent Atom, typ string) *Atom {
	for _, c := range parent.Children {
		if c.Type == typ {
			return &c
		}
	}
	return nil
}

// Helper to read payload
func readPayload(f *os.File, atom *Atom) []byte {
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
func (d *Demuxer) ExtractTracks(moov Atom) ([]Track, error) {
	var tracks []Track

	for _, child := range moov.Children {
		if child.Type == "trak" {
			track, err := d.parseTrack(child)
			if err != nil {
				fmt.Printf("[Demuxer] Warning: Failed to parse track: %v\n", err)
				continue
			}
			tracks = append(tracks, *track)
		}
	}

	if len(tracks) == 0 {
		return nil, fmt.Errorf("no valid tracks found in moov")
	}

	return tracks, nil
}

// parseTrack parses a single 'trak' atom into a Track struct
func (d *Demuxer) parseTrack(trak Atom) (*Track, error) {
	tr := &Track{}

	// 1. tkhd (Track Header)
	tkhdAtom := findChildPath(trak, "tkhd")
	if tkhdAtom == nil {
		return nil, fmt.Errorf("missing tkhd")
	}
	tr.Tkhd = readPayload(d.file, tkhdAtom)
	// Parse Width/Height for Video (Best effort)
	width, height, _ := d.ParseTkhd(*tkhdAtom)
	tr.Width = width
	tr.Height = height

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
			tr.Type = TrackTypeVideo
		case "soun":
			tr.Type = TrackTypeAudio
		case "hint":
			tr.Type = TrackTypeHint
		default:
			tr.Type = TrackTypeMeta
		}
	}

	// 4. mdia -> minf (Media Info)
	minfAtom := findChildPath(*mdiaAtom, "minf")
	if minfAtom == nil {
		return nil, fmt.Errorf("missing minf")
	}

	// Media Header (vmhd or smhd)
	if tr.Type == TrackTypeVideo {
		vmhdAtom := findChildPath(*minfAtom, "vmhd")
		if vmhdAtom != nil {
			tr.MediaHeader = readPayload(d.file, vmhdAtom)
		}
	} else if tr.Type == TrackTypeAudio {
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
		// stsd: Ver(4) + EntryCount(4) + EntrySize(4) + CodecTag(4)
		// The codec tag is at offset 12 within the stsd payload
		tr.CodecTag = string(tr.Stsd[12:16])
		fmt.Printf("[Demuxer] Track %s: Codec Tag = '%s'\n", tr.Type, tr.CodecTag)
	}

	return tr, nil
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
func (d *Demuxer) ParseStts(atom Atom) ([]struct{ Count, Duration uint32 }, error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil { // Skip Header
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
func (d *Demuxer) ParseStss(atom Atom) ([]uint32, error) {
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
func (d *Demuxer) ParseStco(atom Atom) ([]uint32, error) {
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
func (d *Demuxer) ParseStsz(atom Atom) (uint32, []uint32, error) {
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
func (d *Demuxer) ParseStsc(atom Atom) ([]struct{ FirstChunk, SamplesPerChunk, SampleDescID uint32 }, error) {
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
func (d *Demuxer) ParseCtts(atom Atom) ([]struct {
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
			// Version 0: unsigned 32-bit offset
			var uoff uint32
			if err := binary.Read(d.file, binary.BigEndian, &uoff); err != nil {
				return nil, err
			}
			entries[i].Offset = int32(uoff)
		} else {
			// Version 1: signed 32-bit offset
			if err := binary.Read(d.file, binary.BigEndian, &entries[i].Offset); err != nil {
				return nil, err
			}
		}
	}
	return entries, nil
}

// ParseElst parses Edit List box for A/V sync correction
func (d *Demuxer) ParseElst(atom Atom) ([]EditListEntry, error) {
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

	entries := make([]EditListEntry, entryCount)
	for i := 0; i < int(entryCount); i++ {
		if version == 1 {
			// 64-bit: SegmentDuration(8) + MediaTime(8) + Rate(4)
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
			// 32-bit: SegmentDuration(4) + MediaTime(4) + Rate(4)
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
		// Rate: 16.16 fixed point
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
func (d *Demuxer) ParseMdhd(atom Atom) (uint32, uint64, error) {
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
		// 64-bit duration
		if _, err := d.file.Seek(16, io.SeekCurrent); err != nil {
			return 0, 0, err
		} // Skip create/mod
		if err := binary.Read(d.file, binary.BigEndian, &timescale); err != nil {
			return 0, 0, err
		}
		if err := binary.Read(d.file, binary.BigEndian, &duration); err != nil {
			return 0, 0, err
		}
	} else {
		// 32-bit duration
		if _, err := d.file.Seek(8, io.SeekCurrent); err != nil {
			return 0, 0, err
		} // Skip create/mod
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

// ParseTkhd parses Track Header to get Width and Height
func (d *Demuxer) ParseTkhd(atom Atom) (width, height uint32, err error) {
	if _, err := d.file.Seek(atom.Offset+8, io.SeekStart); err != nil {
		return 0, 0, err
	}
	version, _, err := readFullBoxHeader(d.file)
	if err != nil {
		return 0, 0, err
	}

	skip := int64(0)
	if version == 0 {
		skip = 20 + 8 + 8 + 36
	} else {
		skip = 32 + 8 + 8 + 36
	}

	if _, err := d.file.Seek(skip, io.SeekCurrent); err != nil {
		return 0, 0, err
	}

	if err := binary.Read(d.file, binary.BigEndian, &width); err != nil {
		return 0, 0, err
	}
	if err := binary.Read(d.file, binary.BigEndian, &height); err != nil {
		return 0, 0, err
	}

	return width, height, nil
}

// LocateTables finds the stbl children from a trak atom (scoped)
func (d *Demuxer) LocateTables(moov Atom) (stss, stts, stco, stsz, stsc *Atom) {
	var find func(atoms []Atom)
	find = func(atoms []Atom) {
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
func (d *Demuxer) MapSamples(moov Atom) ([]Sample, error) {
	stssAtom, sttsAtom, stcoAtom, stszAtom, stscAtom := d.LocateTables(moov)
	if sttsAtom == nil || stcoAtom == nil || stszAtom == nil || stscAtom == nil {
		return nil, fmt.Errorf("missing critical atom tables (stts, stco, stsz, or stsc)")
	}

	// 1. Parse Tables
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

	// 2. Build Keyframe Map
	isKeyframe := make(map[int]bool)
	if len(stss) == 0 {
		// All are keyframes if stss is missing
	} else {
		for _, id := range stss {
			isKeyframe[int(id)] = true
		}
	}

	// 3. Flatten Samples
	// Total samples logic
	numSamples := 0
	if fixedSize != 0 {
		// Need better logic for fixed size total count? usually implied by stts
		numSamples = 0 // Recalculate from stts
		for _, entry := range stts {
			numSamples += int(entry.Count)
		}
	} else {
		numSamples = len(stsz)
	}

	samples := make([]Sample, numSamples)

	// Fill Times
	currentSample := 0
	currentTime := int64(0)
	for _, entry := range stts {
		for i := 0; i < int(entry.Count); i++ {
			if currentSample < len(samples) {
				samples[currentSample].Time = currentTime
				samples[currentSample].Duration = int64(entry.Duration)
				samples[currentSample].ID = currentSample + 1 // 1-based ID
				currentTime += int64(entry.Duration)
				currentSample++
			}
		}
	}

	// Fill Sizes
	if fixedSize != 0 {
		for i := range samples {
			samples[i].Size = int64(fixedSize)
		}
	} else {
		for i := 0; i < len(stsz) && i < len(samples); i++ {
			samples[i].Size = int64(stsz[i])
		}
	}

	// Fill Offsets (The tricky part: stsc + stco)
	// Iterate Chunks
	sampleIdx := 0
	for i, chunkOffset := range stco {
		chunkIndex := i + 1 // 1-based

		// Find stsc entry for this chunk
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
