package mux

import (
	"encoding/binary"
	"io"
	"os"
	"sort"

	"cromedia/core"
)

// MP4Muxer handles the multiplexing and construction of MP4 files.
type MP4Muxer struct {
	file   *os.File
	writer *AtomWriter
}

// NewMP4Muxer creates a new MP4Muxer.
func NewMP4Muxer(file *os.File) *MP4Muxer {
	return &MP4Muxer{
		file:   file,
		writer: &AtomWriter{w: file},
	}
}

// WriteHeader writes ftyp, moov, and the mdat header.
func (m *MP4Muxer) WriteHeader(tracks []core.Track) error {
	// 1. Write ftyp
	ftypSize := uint32(24)
	m.writer.WriteUint32(ftypSize)
	m.writer.WriteTag("ftyp")
	m.writer.WriteTag("isom")
	m.writer.WriteUint32(512)
	m.writer.WriteTag("isom")
	m.writer.WriteTag("mp41")

	// 2. Build Interleaved Sample Order
	interleaved := BuildInterleavedOrder(tracks)

	// 3. Calculate mdat size
	mdatDataSize := int64(0)
	for _, is := range interleaved {
		mdatDataSize += is.Sample.Size
	}

	// 4. Determine if we need co64 (offsets > 4GB)
	useCo64 := mdatDataSize > (1 << 31) // 2GB threshold for safety

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
		_ = is
	}

	// 8. Generate REAL moov with correct offsets
	moov := makeMoovMultiTrackWithOffsets(tracks, interleaved, offsets, useCo64)
	moovBytes := serializeAtom(moov)

	// 9. Write moov
	m.writer.WriteBytes(moovBytes)

	// 10. Write mdat header
	m.writer.WriteUint32(uint32(mdatDataSize + 8))
	m.writer.WriteTag("mdat")

	return nil
}

// WritePacket writes a packet's payload data directly into the mdat atom.
func (m *MP4Muxer) WritePacket(pkt *core.Packet) error {
	m.writer.WriteBytes(pkt.Data)
	return nil
}

// WriteTrailer is a no-op for MP4 (offsets are pre-written).
func (m *MP4Muxer) WriteTrailer() error {
	return nil
}

// Close closes the file handle.
func (m *MP4Muxer) Close() error {
	return m.file.Close()
}

// WriteMultiTrackFile performs a copy-mode remux from the input file based on track samples.
func (m *MP4Muxer) WriteMultiTrackFile(tracks []core.Track, inputFile *os.File) error {
	err := m.WriteHeader(tracks)
	if err != nil {
		return err
	}

	interleaved := BuildInterleavedOrder(tracks)
	
	// Pre-allocate a 64KB buffer from the pool to stream data block-by-block
	bufSize := 65536
	copyBuf := core.GlobalGet(bufSize)
	defer core.GlobalPut(copyBuf)

	for _, is := range interleaved {
		_, err := inputFile.Seek(is.Sample.Offset, io.SeekStart)
		if err != nil {
			return err
		}

		remaining := is.Sample.Size
		for remaining > 0 {
			toRead := int64(bufSize)
			if remaining < toRead {
				toRead = remaining
			}

			_, err = io.ReadFull(inputFile, copyBuf[:toRead])
			if err != nil {
				return err
			}

			pkt := &core.Packet{Data: copyBuf[:toRead]}
			err = m.WritePacket(pkt)
			if err != nil {
				return err
			}
			remaining -= toRead
		}
	}

	return m.WriteTrailer()
}

func BuildInterleavedOrder(tracks []core.Track) []core.InterleavedSample {
	var all []core.InterleavedSample

	for ti, t := range tracks {
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

func makeMoovMultiTrack(tracks []core.Track, interleaved []core.InterleavedSample, baseOffset int64, useCo64 bool) *SimpleAtom {
	dummyOffsets := make([]int64, len(interleaved))
	for i := range dummyOffsets {
		dummyOffsets[i] = baseOffset
	}
	return makeMoovMultiTrackWithOffsets(tracks, interleaved, dummyOffsets, useCo64)
}

func makeMoovMultiTrackWithOffsets(tracks []core.Track, interleaved []core.InterleavedSample, offsets []int64, useCo64 bool) *SimpleAtom {
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
	mvhdData.WriteUint32(0)
	mvhdData.WriteUint32(0)
	mvhdData.WriteUint32(0)
	mvhdData.WriteUint32(mvhdTimescale)
	mvhdData.WriteUint32(uint32(maxDuration))
	mvhdData.WriteUint32(0x00010000)
	mvhdData.WriteUint16(0x0100)
	mvhdData.WriteBytes(make([]byte, 10))
	mvhdData.WriteBytes(identityMatrix())
	mvhdData.WriteBytes(make([]byte, 24))
	mvhdData.WriteUint32(uint32(len(tracks) + 1))

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

func makeTrakAtom(t core.Track, trackID int, sampleOffsets map[int]int64, useCo64 bool) *SimpleAtom {
	numSamples := len(t.Samples)

	sttsData := new(ExcludeBuffer)
	sttsData.WriteUint32(0)
	sttsData.WriteUint32(uint32(numSamples))
	for _, s := range t.Samples {
		sttsData.WriteUint32(1)
		sttsData.WriteUint32(uint32(s.Duration))
	}

	stszData := new(ExcludeBuffer)
	stszData.WriteUint32(0)
	stszData.WriteUint32(0)
	stszData.WriteUint32(uint32(numSamples))
	for _, s := range t.Samples {
		stszData.WriteUint32(uint32(s.Size))
	}

	var chunkOffsetAtom *SimpleAtom
	if useCo64 {
		co64Data := new(ExcludeBuffer)
		co64Data.WriteUint32(0)
		co64Data.WriteUint32(uint32(numSamples))
		for i := 0; i < numSamples; i++ {
			off := sampleOffsets[i]
			co64Data.WriteUint32(uint32(off >> 32))
			co64Data.WriteUint32(uint32(off))
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

	stscData := new(ExcludeBuffer)
	stscData.WriteUint32(0)
	stscData.WriteUint32(1)
	stscData.WriteUint32(1)
	stscData.WriteUint32(1)
	stscData.WriteUint32(1)

	var stssAtom *SimpleAtom
	if t.Type == core.TrackTypeVideo {
		var keyframes []int
		for i, s := range t.Samples {
			if s.IsKeyframe {
				keyframes = append(keyframes, i+1)
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

	var cttsAtom *SimpleAtom
	if len(t.CTSOffsets) > 0 {
		cttsBuf := new(ExcludeBuffer)
		cttsBuf.WriteUint32(0)
		cttsBuf.WriteUint32(uint32(len(t.CTSOffsets)))
		for _, off := range t.CTSOffsets {
			cttsBuf.WriteUint32(1)
			cttsBuf.WriteUint32(uint32(off))
		}
		cttsAtom = &SimpleAtom{Type: "ctts", Data: cttsBuf.Bytes()}
	}

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

	minfChildren := []*SimpleAtom{}
	if t.MediaHeader != nil {
		headerType := "vmhd"
		if t.Type == core.TrackTypeAudio {
			headerType = "smhd"
		}
		minfChildren = append(minfChildren, &SimpleAtom{Type: headerType, Data: t.MediaHeader})
	}
	dinf := &SimpleAtom{Type: "dinf", Children: []*SimpleAtom{
		{Type: "dref", Data: []byte{
			0, 0, 0, 0,
			0, 0, 0, 1,
			0, 0, 0, 12, 117, 114, 108, 32, 0, 0, 0, 1,
		}},
	}}
	minfChildren = append(minfChildren, dinf, stbl)
	minf := &SimpleAtom{Type: "minf", Children: minfChildren}

	totalDur := int64(0)
	for _, s := range t.Samples {
		totalDur += s.Duration
	}

	mdhdData := new(ExcludeBuffer)
	mdhdData.WriteUint32(0)
	mdhdData.WriteUint32(0)
	mdhdData.WriteUint32(0)
	mdhdData.WriteUint32(t.Timescale)
	mdhdData.WriteUint32(uint32(totalDur))
	mdhdData.WriteUint16(0x55c4)
	mdhdData.WriteUint16(0)

	mdia := &SimpleAtom{Type: "mdia", Children: []*SimpleAtom{
		{Type: "mdhd", Data: mdhdData.Bytes()},
		{Type: "hdlr", Data: t.Hdlr},
		minf,
	}}

	tkhdData := new(ExcludeBuffer)
	tkhdData.WriteUint32(0x00000003)
	tkhdData.WriteUint32(0)
	tkhdData.WriteUint32(0)
	tkhdData.WriteUint32(uint32(trackID))
	tkhdData.WriteUint32(0)
	durMvhd := convertTime(uint64(totalDur), t.Timescale, 1000)
	tkhdData.WriteUint32(uint32(durMvhd))
	tkhdData.WriteUint32(0)
	tkhdData.WriteUint32(0)
	tkhdData.WriteUint16(0)
	tkhdData.WriteUint16(0)
	vol := uint16(0)
	if t.Type == core.TrackTypeAudio {
		vol = 0x0100
	}
	tkhdData.WriteUint16(vol)
	tkhdData.WriteUint16(0)
	if len(t.Matrix) == 36 {
		tkhdData.WriteBytes(t.Matrix)
	} else {
		tkhdData.WriteBytes(identityMatrix())
	}
	tkhdData.WriteUint32(t.Width)
	tkhdData.WriteUint32(t.Height)

	trakChildren := []*SimpleAtom{
		{Type: "tkhd", Data: tkhdData.Bytes()},
	}

	if len(t.EditList) > 0 {
		elstData := new(ExcludeBuffer)
		elstData.WriteUint32(0)
		elstData.WriteUint32(uint32(len(t.EditList)))
		for _, e := range t.EditList {
			elstData.WriteUint32(uint32(e.SegmentDuration))
			elstData.WriteUint32(uint32(e.MediaTime))
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
