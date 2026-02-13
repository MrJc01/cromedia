package core

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
)

// Remuxer handles the reconstruction of MP4 atoms
type Remuxer struct {
	InputFile *os.File
}

// WriteMultiTrackFile generates a valid MP4 from a list of Tracks with interleaved mdat
func (r *Remuxer) WriteMultiTrackFile(outputFile string, tracks []Track) error {
	out, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer out.Close()

	writer := &AtomWriter{w: out}

	// 1. Write ftyp
	ftypSize := uint32(24)
	writer.WriteUint32(ftypSize)
	writer.WriteTag("ftyp")
	writer.WriteTag("isom")
	writer.WriteUint32(512)
	writer.WriteTag("isom")
	writer.WriteTag("mp41")

	// 2. Build Interleaved Sample Order
	interleaved := buildInterleavedOrder(tracks)
	fmt.Printf("[Remuxer] Interleaved %d total samples across %d tracks\n", len(interleaved), len(tracks))

	// 3. Calculate mdat size
	mdatDataSize := int64(0)
	for _, is := range interleaved {
		mdatDataSize += is.Sample.Size
	}

	// 4. Determine if we need co64 (offsets > 4GB)
	useCo64 := mdatDataSize > (1 << 31) // Conservative: 2GB threshold for safety

	// 5. Generate moov with dummy offsets to calculate its size
	dummyMoov := makeMoovMultiTrack(tracks, interleaved, 0, useCo64)
	dummyBytes := serializeAtom(dummyMoov)

	// 6. Calculate real mdat start position
	mdatStartPos := int64(ftypSize) + int64(len(dummyBytes)) + 8 // +8 for mdat header

	// 7. Calculate real offsets per sample based on interleaved order
	offsets := make([]int64, len(interleaved))
	currentPos := mdatStartPos
	for i, is := range interleaved {
		offsets[i] = currentPos
		currentPos += is.Sample.Size
		_ = is // used below
	}

	// 8. Generate REAL moov with correct offsets
	moov := makeMoovMultiTrackWithOffsets(tracks, interleaved, offsets, useCo64)
	moovBytes := serializeAtom(moov)

	// 9. Write moov
	writer.WriteBytes(moovBytes)

	// 10. Write mdat header
	writer.WriteUint32(uint32(mdatDataSize + 8))
	writer.WriteTag("mdat")

	// 11. Write mdat body (INTERLEAVED!)
	copyBuffer := make([]byte, 1024*1024)
	fmt.Printf("[Remuxer] Writing interleaved mdat (%d bytes)...\n", mdatDataSize)

	for _, is := range interleaved {
		_, err := r.InputFile.Seek(is.Sample.Offset, 0)
		if err != nil {
			return fmt.Errorf("seek error at offset %d: %w", is.Sample.Offset, err)
		}
		limitReader := io.LimitReader(r.InputFile, is.Sample.Size)
		_, err = io.CopyBuffer(out, limitReader, copyBuffer)
		if err != nil {
			return fmt.Errorf("copy error: %w", err)
		}
	}

	return nil
}

// buildInterleavedOrder creates a sorted list of all samples across all tracks,
// ordered by presentation time in seconds. This ensures audio and video chunks
// are naturally interleaved for streaming playback.
func buildInterleavedOrder(tracks []Track) []InterleavedSample {
	var all []InterleavedSample

	for ti, t := range tracks {
		ts := float64(t.Timescale)
		if ts == 0 {
			ts = 1000
		}
		for si, s := range t.Samples {
			timeSeconds := float64(s.Time) / ts
			all = append(all, InterleavedSample{
				TrackIndex:  ti,
				SampleIndex: si,
				TimeSeconds: timeSeconds,
				Sample:      s,
			})
		}
	}

	// Sort by time, then by track index (video first if same time)
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].TimeSeconds != all[j].TimeSeconds {
			return all[i].TimeSeconds < all[j].TimeSeconds
		}
		return all[i].TrackIndex < all[j].TrackIndex
	})

	return all
}

// makeMoovMultiTrack creates a moov atom with dummy offset 0 (for size calculation)
func makeMoovMultiTrack(tracks []Track, interleaved []InterleavedSample, baseOffset int64, useCo64 bool) *SimpleAtom {
	dummyOffsets := make([]int64, len(interleaved))
	for i := range dummyOffsets {
		dummyOffsets[i] = baseOffset
	}
	return makeMoovMultiTrackWithOffsets(tracks, interleaved, dummyOffsets, useCo64)
}

// makeMoovMultiTrackWithOffsets creates moov with real offsets from interleaved order
func makeMoovMultiTrackWithOffsets(tracks []Track, interleaved []InterleavedSample, offsets []int64, useCo64 bool) *SimpleAtom {
	// Build per-track offset maps: trackIndex -> sampleIndex -> offset
	trackOffsets := make(map[int]map[int]int64)
	for i, is := range interleaved {
		if trackOffsets[is.TrackIndex] == nil {
			trackOffsets[is.TrackIndex] = make(map[int]int64)
		}
		trackOffsets[is.TrackIndex][is.SampleIndex] = offsets[i]
	}

	var traks []*SimpleAtom
	for i, t := range tracks {
		sampleOffsets := trackOffsets[i]
		trak := makeTrakAtom(t, i+1, sampleOffsets, useCo64)
		traks = append(traks, trak)
	}

	// mvhd
	mvhdTimescale := uint32(1000)
	maxDuration := int64(0)
	for _, t := range tracks {
		totalDur := int64(0)
		for _, s := range t.Samples {
			totalDur += s.Duration
		}
		dur := convertTime(uint64(totalDur), t.Timescale, mvhdTimescale)
		if dur > maxDuration {
			maxDuration = dur
		}
	}

	mvhdData := new(ExcludeBuffer)
	mvhdData.WriteUint32(0) // Version + Flags
	mvhdData.WriteUint32(0) // Creation
	mvhdData.WriteUint32(0) // Modification
	mvhdData.WriteUint32(mvhdTimescale)
	mvhdData.WriteUint32(uint32(maxDuration))
	mvhdData.WriteUint32(0x00010000)      // Rate (1.0)
	mvhdData.WriteUint16(0x0100)          // Volume (1.0)
	mvhdData.WriteBytes(make([]byte, 10)) // Reserved
	mvhdData.WriteBytes(identityMatrix())
	mvhdData.WriteBytes(make([]byte, 24))         // Pre-defined
	mvhdData.WriteUint32(uint32(len(tracks) + 1)) // Next Track ID

	children := []*SimpleAtom{{Type: "mvhd", Data: mvhdData.Bytes()}}
	children = append(children, traks...)

	return &SimpleAtom{Type: "moov", Children: children}
}

func identityMatrix() []byte {
	return []byte{
		0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 64, 0, 0, 0,
	}
}

func makeTrakAtom(t Track, trackID int, sampleOffsets map[int]int64, useCo64 bool) *SimpleAtom {
	numSamples := len(t.Samples)

	// 1. stts (Time-to-Sample)
	sttsData := new(ExcludeBuffer)
	sttsData.WriteUint32(0) // Version + Flags
	sttsData.WriteUint32(uint32(numSamples))
	for _, s := range t.Samples {
		sttsData.WriteUint32(1)
		sttsData.WriteUint32(uint32(s.Duration))
	}

	// 2. stsz (Sample Sizes)
	stszData := new(ExcludeBuffer)
	stszData.WriteUint32(0) // Version + Flags
	stszData.WriteUint32(0) // Default size (0 = variable)
	stszData.WriteUint32(uint32(numSamples))
	for _, s := range t.Samples {
		stszData.WriteUint32(uint32(s.Size))
	}

	// 3. stco/co64 (Chunk Offsets) - Using interleaved offsets!
	var chunkOffsetAtom *SimpleAtom
	if useCo64 {
		co64Data := new(ExcludeBuffer)
		co64Data.WriteUint32(0)
		co64Data.WriteUint32(uint32(numSamples))
		for i := 0; i < numSamples; i++ {
			off := sampleOffsets[i]
			co64Data.WriteUint32(uint32(off >> 32)) // High 32
			co64Data.WriteUint32(uint32(off))       // Low 32
		}
		chunkOffsetAtom = &SimpleAtom{Type: "co64", Data: co64Data.Bytes()}
	} else {
		stcoData := new(ExcludeBuffer)
		stcoData.WriteUint32(0)
		stcoData.WriteUint32(uint32(numSamples))
		for i := 0; i < numSamples; i++ {
			stcoData.WriteUint32(uint32(sampleOffsets[i]))
		}
		chunkOffsetAtom = &SimpleAtom{Type: "stco", Data: stcoData.Bytes()}
	}

	// 4. stsc (Sample-to-Chunk)
	stscData := new(ExcludeBuffer)
	stscData.WriteUint32(0) // Version + Flags
	stscData.WriteUint32(1) // Entry count
	stscData.WriteUint32(1) // First Chunk
	stscData.WriteUint32(1) // Samples Per Chunk (1:1 map for interleaving)
	stscData.WriteUint32(1) // Sample Description ID

	// 5. stss (Sync Samples / Keyframes) - Video only
	var stssAtom *SimpleAtom
	if t.Type == TrackTypeVideo {
		var keyframes []int
		for i, s := range t.Samples {
			if s.IsKeyframe {
				keyframes = append(keyframes, i+1) // 1-based
			}
		}
		stssBuf := new(ExcludeBuffer)
		stssBuf.WriteUint32(0)
		stssBuf.WriteUint32(uint32(len(keyframes)))
		for _, kf := range keyframes {
			stssBuf.WriteUint32(uint32(kf))
		}
		stssAtom = &SimpleAtom{Type: "stss", Data: stssBuf.Bytes()}
	}

	// 6. ctts (Composition Time to Sample) - B-Frame support
	var cttsAtom *SimpleAtom
	if len(t.CTSOffsets) > 0 {
		cttsBuf := new(ExcludeBuffer)
		cttsBuf.WriteUint32(0) // Version 0 + Flags
		cttsBuf.WriteUint32(uint32(len(t.CTSOffsets)))
		for _, off := range t.CTSOffsets {
			cttsBuf.WriteUint32(1) // Count = 1 per entry (expanded)
			cttsBuf.WriteUint32(uint32(off))
		}
		cttsAtom = &SimpleAtom{Type: "ctts", Data: cttsBuf.Bytes()}
	}

	// Build stbl
	stblChildren := []*SimpleAtom{
		{Type: "stsd", Data: t.Stsd},
		{Type: "stts", Data: sttsData.Bytes()},
		{Type: "stsz", Data: stszData.Bytes()},
		chunkOffsetAtom,
		{Type: "stsc", Data: stscData.Bytes()},
	}
	if stssAtom != nil {
		stblChildren = append(stblChildren, stssAtom)
	}
	if cttsAtom != nil {
		stblChildren = append(stblChildren, cttsAtom)
	}
	stbl := &SimpleAtom{Type: "stbl", Children: stblChildren}

	// minf
	minfChildren := []*SimpleAtom{}
	if t.MediaHeader != nil {
		headerType := "vmhd"
		if t.Type == TrackTypeAudio {
			headerType = "smhd"
		}
		minfChildren = append(minfChildren, &SimpleAtom{Type: headerType, Data: t.MediaHeader})
	}
	dinf := &SimpleAtom{Type: "dinf", Children: []*SimpleAtom{
		{Type: "dref", Data: []byte{
			0, 0, 0, 0, // Version + Flags
			0, 0, 0, 1, // Entry count
			0, 0, 0, 12, 117, 114, 108, 32, 0, 0, 0, 1, // url entry
		}},
	}}
	minfChildren = append(minfChildren, dinf, stbl)
	minf := &SimpleAtom{Type: "minf", Children: minfChildren}

	// mdia
	totalDur := int64(0)
	for _, s := range t.Samples {
		totalDur += s.Duration
	}

	mdhdData := new(ExcludeBuffer)
	mdhdData.WriteUint32(0)           // Version + Flags
	mdhdData.WriteUint32(0)           // Creation
	mdhdData.WriteUint32(0)           // Modification
	mdhdData.WriteUint32(t.Timescale) // Timescale
	mdhdData.WriteUint32(uint32(totalDur))
	mdhdData.WriteUint16(0x55c4) // Language (undetermined)
	mdhdData.WriteUint16(0)      // Quality

	mdia := &SimpleAtom{Type: "mdia", Children: []*SimpleAtom{
		{Type: "mdhd", Data: mdhdData.Bytes()},
		{Type: "hdlr", Data: t.Hdlr},
		minf,
	}}

	// tkhd
	tkhdData := new(ExcludeBuffer)
	tkhdData.WriteUint32(0x00000003) // Flags: Enabled(1) + InMovie(2)
	tkhdData.WriteUint32(0)          // Creation
	tkhdData.WriteUint32(0)          // Modification
	tkhdData.WriteUint32(uint32(trackID))
	tkhdData.WriteUint32(0) // Reserved
	durMvhd := convertTime(uint64(totalDur), t.Timescale, 1000)
	tkhdData.WriteUint32(uint32(durMvhd))
	tkhdData.WriteUint32(0) // Reserved
	tkhdData.WriteUint32(0) // Reserved
	tkhdData.WriteUint16(0) // Layer
	tkhdData.WriteUint16(0) // Alternate Group
	vol := uint16(0)
	if t.Type == TrackTypeAudio {
		vol = 0x0100
	}
	tkhdData.WriteUint16(vol) // Volume
	tkhdData.WriteUint16(0)   // Reserved
	tkhdData.WriteBytes(identityMatrix())
	tkhdData.WriteUint32(t.Width)
	tkhdData.WriteUint32(t.Height)

	// Build trak children
	trakChildren := []*SimpleAtom{
		{Type: "tkhd", Data: tkhdData.Bytes()},
	}

	// edts (Edit List) — Sync correction propagation
	if len(t.EditList) > 0 {
		elstData := new(ExcludeBuffer)
		elstData.WriteUint32(0) // Version 0 + Flags
		elstData.WriteUint32(uint32(len(t.EditList)))
		for _, e := range t.EditList {
			elstData.WriteUint32(uint32(e.SegmentDuration))
			elstData.WriteUint32(uint32(e.MediaTime)) // int32 in v0
			elstData.WriteUint16(uint16(e.MediaRateInt))
			elstData.WriteUint16(uint16(e.MediaRateFrac))
		}
		edts := &SimpleAtom{Type: "edts", Children: []*SimpleAtom{
			{Type: "elst", Data: elstData.Bytes()},
		}}
		trakChildren = append(trakChildren, edts)
	}

	trakChildren = append(trakChildren, mdia)

	return &SimpleAtom{Type: "trak", Children: trakChildren}
}

func convertTime(val uint64, fromScale, toScale uint32) int64 {
	if fromScale == 0 {
		return 0
	}
	return int64(val) * int64(toScale) / int64(fromScale)
}

// --- Atom Writer Helpers ---

type AtomWriter struct {
	w io.Writer
}

func (w *AtomWriter) WriteUint32(val uint32) {
	binary.Write(w.w, binary.BigEndian, val)
}

func (w *AtomWriter) WriteUint16(val uint16) {
	binary.Write(w.w, binary.BigEndian, val)
}

func (w *AtomWriter) WriteTag(tag string) {
	w.w.Write([]byte(tag))
}

func (w *AtomWriter) WriteBytes(b []byte) {
	w.w.Write(b)
}

type ExcludeBuffer struct {
	buf []byte
}

func (b *ExcludeBuffer) WriteUint32(val uint32) {
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, val)
	b.buf = append(b.buf, tmp...)
}

func (b *ExcludeBuffer) WriteUint16(val uint16) {
	tmp := make([]byte, 2)
	binary.BigEndian.PutUint16(tmp, val)
	b.buf = append(b.buf, tmp...)
}

func (b *ExcludeBuffer) WriteBytes(data []byte) {
	b.buf = append(b.buf, data...)
}

func (b *ExcludeBuffer) Bytes() []byte {
	return b.buf
}

type SimpleAtom struct {
	Type     string
	Data     []byte
	Children []*SimpleAtom
}

func serializeAtom(atom *SimpleAtom) []byte {
	var childBytes []byte
	for _, c := range atom.Children {
		childBytes = append(childBytes, serializeAtom(c)...)
	}

	totalSize := 8 + len(atom.Data) + len(childBytes)

	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:], uint32(totalSize))
	copy(buf[4:], atom.Type)

	res := append(buf, atom.Data...)
	res = append(res, childBytes...)
	return res
}
