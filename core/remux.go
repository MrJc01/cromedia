package core

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Remuxer handles the reconstruction of MP4 atoms
type Remuxer struct {
	InputFile *os.File
}

// WriteMultiTrackFile generates a valid MP4 from a list of Tracks
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
	writer.WriteTag("isom") // Major brand
	writer.WriteUint32(512) // Minor version
	writer.WriteTag("isom") // Compatible
	writer.WriteTag("mp41") // Compatible

	// 2. Prepare Metadata (moov)
	// a. Generate moov with dummy offsets (0)
	dummyMoov := makeMoovMultiTrack(tracks, 0)
	dummyBytes := serializeAtom(dummyMoov)

	// b. Calculate mdat position
	// ftyp(24) + moov(len(dummyBytes)) + mdatHeader(8)
	mdatStartPos := int64(ftypSize) + int64(len(dummyBytes)) + 8

	// c. Generate REAL moov with correct offsets
	// For MVP: Sequential. Track 0 samples, then Track 1 samples...
	moov := makeMoovMultiTrack(tracks, mdatStartPos)
	moovBytes := serializeAtom(moov)

	// 3. Write 'moov'
	writer.WriteBytes(moovBytes)

	// 4. Write 'mdat' Header
	mdatDataSize := int64(0)
	for _, t := range tracks {
		for _, s := range t.Samples {
			mdatDataSize += s.Size
		}
	}
	writer.WriteUint32(uint32(mdatDataSize + 8)) // Atom Size
	writer.WriteTag("mdat")

	// 5. Write 'mdat' Body
	copyBuffer := make([]byte, 1024*1024) // 1MB buffer

	fmt.Printf("[Remuxer] Writing mdat (Sequential strategy)...\n")

	for _, t := range tracks {
		fmt.Printf("  -> Writing Track %s (%d samples)\n", t.Type, len(t.Samples))
		for _, s := range t.Samples {
			// Seek to original sample
			_, err := r.InputFile.Seek(s.Offset, 0)
			if err != nil {
				return err
			}

			// Copy s.Size
			limitReader := io.LimitReader(r.InputFile, s.Size)
			_, err = io.CopyBuffer(out, limitReader, copyBuffer)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func makeMoovMultiTrack(tracks []Track, mdatOffsetBase int64) *SimpleAtom {
	var traks []*SimpleAtom

	currentOffset := mdatOffsetBase

	for i, t := range tracks {
		trak := makeTrakAtom(t, i+1, currentOffset)
		traks = append(traks, trak)

		// Advance offset by size of this track (Sequential)
		trackSize := int64(0)
		for _, s := range t.Samples {
			trackSize += s.Size
		}
		currentOffset += trackSize
	}

	// mvhd
	mvhdTimescale := uint32(1000)
	maxDuration := int64(0)
	for _, t := range tracks {
		dur := convertTime(t.Duration, t.Timescale, mvhdTimescale)
		if dur > maxDuration {
			func(d int64) { maxDuration = d }(dur)
		} // lambda fix? Just assign.
		maxDuration = dur // Override logic: we want Max.
		// NOTE: Loop logic above is broken, fixing:
	}

	// Fix max duration calc
	maxDuration = 0
	for _, t := range tracks {
		// Calculate total duration of samples just to be safe?
		// Or use t.Duration (whole track) scaled?
		// Better: sum of samples duration
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
	mvhdData.WriteUint32(mvhdTimescale) // Timescale
	mvhdData.WriteUint32(uint32(maxDuration))
	mvhdData.WriteUint32(0x00010000)      // Rate
	mvhdData.WriteUint16(0x0100)          // Volume
	mvhdData.WriteBytes(make([]byte, 10)) // Reserved
	// Matrix
	matrix := []byte{
		0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 64, 0, 0, 0,
	}
	mvhdData.WriteBytes(matrix)
	mvhdData.WriteBytes(make([]byte, 24))         // Pre-defined
	mvhdData.WriteUint32(uint32(len(tracks) + 1)) // Next Track ID

	children := []*SimpleAtom{{Type: "mvhd", Data: mvhdData.Bytes()}}
	children = append(children, traks...)

	return &SimpleAtom{Type: "moov", Children: children}
}

func makeTrakAtom(t Track, trackID int, startOffset int64) *SimpleAtom {
	// 1. stts
	sttsData := new(ExcludeBuffer)
	sttsData.WriteUint32(0)
	sttsData.WriteUint32(uint32(len(t.Samples)))
	for _, s := range t.Samples {
		sttsData.WriteUint32(1)
		sttsData.WriteUint32(uint32(s.Duration))
	}

	// 2. stsz
	stszData := new(ExcludeBuffer)
	stszData.WriteUint32(0)
	stszData.WriteUint32(0)
	stszData.WriteUint32(uint32(len(t.Samples)))
	for _, s := range t.Samples {
		stszData.WriteUint32(uint32(s.Size))
	}

	// 3. stco (Offsets)
	stcoData := new(ExcludeBuffer)
	stcoData.WriteUint32(0)
	stcoData.WriteUint32(uint32(len(t.Samples)))

	curr := startOffset
	for _, s := range t.Samples {
		stcoData.WriteUint32(uint32(curr))
		curr += s.Size
	}

	// 4. stsc
	stscData := new(ExcludeBuffer)
	stscData.WriteUint32(0)
	stscData.WriteUint32(1)
	stscData.WriteUint32(1) // First Chunk
	stscData.WriteUint32(1) // Samples Per Chunk
	stscData.WriteUint32(1) // ID

	// 5. stss (Sync)
	var stss *SimpleAtom
	if t.Type == TrackTypeVideo {
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
		stss = &SimpleAtom{Type: "stss", Data: stssBuf.Bytes()}
	}

	// Build stbl
	stblChildren := []*SimpleAtom{
		{Type: "stsd", Data: t.Stsd},
		{Type: "stts", Data: sttsData.Bytes()},
		{Type: "stsz", Data: stszData.Bytes()},
		{Type: "stco", Data: stcoData.Bytes()},
		{Type: "stsc", Data: stscData.Bytes()},
	}
	if stss != nil {
		stblChildren = append(stblChildren, stss)
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
			0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 12, 117, 114, 108, 32, 0, 0, 0, 1,
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
	mdhdData.WriteUint32(0)
	mdhdData.WriteUint32(0)
	mdhdData.WriteUint32(0)
	mdhdData.WriteUint32(t.Timescale)
	mdhdData.WriteUint32(uint32(totalDur))
	mdhdData.WriteUint16(0x55c4) // Lang
	mdhdData.WriteUint16(0)

	mdia := &SimpleAtom{Type: "mdia", Children: []*SimpleAtom{
		{Type: "mdhd", Data: mdhdData.Bytes()},
		{Type: "hdlr", Data: t.Hdlr},
		minf,
	}}

	// tkhd
	tkhdData := new(ExcludeBuffer)
	tkhdData.WriteUint32(0x00000001 + 2) // Enabled + InMovie
	tkhdData.WriteUint32(0)
	tkhdData.WriteUint32(0)
	tkhdData.WriteUint32(uint32(trackID))
	tkhdData.WriteUint32(0)
	durMvhd := convertTime(uint64(totalDur), t.Timescale, 1000)
	tkhdData.WriteUint32(uint32(durMvhd))
	tkhdData.WriteUint32(0)
	tkhdData.WriteUint32(0)
	tkhdData.WriteUint16(0) // Layer
	tkhdData.WriteUint16(0) // Alt

	vol := uint16(0)
	if t.Type == TrackTypeAudio {
		vol = 0x0100
	}
	tkhdData.WriteUint16(vol)

	tkhdData.WriteUint16(0)

	matrix := []byte{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 0, 0, 0}
	tkhdData.WriteBytes(matrix)
	tkhdData.WriteUint32(t.Width)
	tkhdData.WriteUint32(t.Height)

	return &SimpleAtom{Type: "trak", Children: []*SimpleAtom{
		{Type: "tkhd", Data: tkhdData.Bytes()},
		mdia,
	}}
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
	// Calculate size: 8 + len(Data) + sum(Children)
	totalSize := 8 + len(atom.Data)
	// for _, c := range atom.Children {
	// Efficiency: In a real implementation we might pre-calculate
	// }

	// We need buffers for children
	var childBytes []byte
	for _, c := range atom.Children {
		childBytes = append(childBytes, serializeAtom(c)...)
	}

	totalSize += len(childBytes)

	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:], uint32(totalSize))
	copy(buf[4:], atom.Type)

	res := append(buf, atom.Data...)
	res = append(res, childBytes...)
	return res
}
