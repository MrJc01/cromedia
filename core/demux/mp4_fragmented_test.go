package demux

import (
	"encoding/binary"
	"os"
	"testing"

	"cromedia/core"
)

func makeAtom(typ string, payload []byte) []byte {
	size := len(payload) + 8
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], uint32(size))
	copy(buf[4:8], typ)
	return append(buf, payload...)
}

func TestFragmentedMP4Demuxer(t *testing.T) {
	// Build a valid fMP4 in-memory byte sequence
	ftyp := makeAtom("ftyp", []byte("mp42\x00\x00\x00\x00mp42isom"))

	// 1. trak -> tkhd
	tkhdPayload := make([]byte, 84)
	binary.BigEndian.PutUint32(tkhdPayload[12:16], 1)   // trackID = 1
	binary.BigEndian.PutUint32(tkhdPayload[76:80], 320) // width
	binary.BigEndian.PutUint32(tkhdPayload[80:84], 240) // height
	tkhd := makeAtom("tkhd", tkhdPayload)

	// 2. trak -> mdia -> mdhd
	mdhdPayload := make([]byte, 20)
	binary.BigEndian.PutUint32(mdhdPayload[12:16], 1000) // timescale = 1000
	mdhd := makeAtom("mdhd", mdhdPayload)

	// 3. trak -> mdia -> hdlr
	hdlrPayload := make([]byte, 24)
	copy(hdlrPayload[8:12], "vide")
	hdlr := makeAtom("hdlr", hdlrPayload)

	// 4. trak -> mdia -> minf -> vmhd
	vmhd := makeAtom("vmhd", make([]byte, 12))

	// 5. trak -> mdia -> minf -> dinf -> dref
	url := makeAtom("url ", []byte{0, 0, 0, 1})
	drefPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(drefPayload[4:8], 1)
	dref := makeAtom("dref", append(drefPayload, url...))
	dinf := makeAtom("dinf", dref)

	// 6. trak -> mdia -> minf -> stbl -> tables (empty for fMP4)
	stsdPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(stsdPayload[4:8], 1)
	avc1Payload := make([]byte, 78)
	copy(avc1Payload[4:8], "avc1")
	stsd := makeAtom("stsd", append(stsdPayload, makeAtom("avc1", avc1Payload)...))

	stts := makeAtom("stts", make([]byte, 8))
	stsc := makeAtom("stsc", make([]byte, 8))
	stsz := makeAtom("stsz", make([]byte, 12))
	stco := makeAtom("stco", make([]byte, 8))

	var stblBytes []byte
	stblBytes = append(stblBytes, stsd...)
	stblBytes = append(stblBytes, stts...)
	stblBytes = append(stblBytes, stsc...)
	stblBytes = append(stblBytes, stsz...)
	stblBytes = append(stblBytes, stco...)
	stbl := makeAtom("stbl", stblBytes)

	var minfBytes []byte
	minfBytes = append(minfBytes, vmhd...)
	minfBytes = append(minfBytes, dinf...)
	minfBytes = append(minfBytes, stbl...)
	minf := makeAtom("minf", minfBytes)

	var mdiaBytes []byte
	mdiaBytes = append(mdiaBytes, mdhd...)
	mdiaBytes = append(mdiaBytes, hdlr...)
	mdiaBytes = append(mdiaBytes, minf...)
	mdia := makeAtom("mdia", mdiaBytes)

	var trakBytes []byte
	trakBytes = append(trakBytes, tkhd...)
	trakBytes = append(trakBytes, mdia...)
	trak := makeAtom("trak", trakBytes)
	moov := makeAtom("moov", trak)

	// 7. moof (fragment)
	mfhd := makeAtom("mfhd", []byte{0, 0, 0, 0, 0, 0, 0, 1})

	// tfhd: default-sample-duration + default-sample-size + default-base-is-moof
	tfhdPayload := make([]byte, 16)
	tfhdPayload[3] = 0x38                               // flags
	binary.BigEndian.PutUint32(tfhdPayload[4:8], 1)     // trackID = 1
	binary.BigEndian.PutUint32(tfhdPayload[8:12], 100)  // defaultDuration = 100
	binary.BigEndian.PutUint32(tfhdPayload[12:16], 500) // defaultSize = 500
	tfhd := makeAtom("tfhd", tfhdPayload)

	// tfdt: decode time = 50
	tfdtPayload := []byte{0, 0, 0, 0, 0, 0, 0, 50}
	tfdt := makeAtom("tfdt", tfdtPayload)

	// trun: data-offset-present (0x01) + sample-size-present (0x0200)
	trunHeader := make([]byte, 12)
	trunHeader[2] = 0x02
	trunHeader[3] = 0x01
	binary.BigEndian.PutUint32(trunHeader[4:8], 2)  // 2 samples
	binary.BigEndian.PutUint32(trunHeader[8:12], 8) // dataOffset relative to moof start = 8

	sample1Size := make([]byte, 4)
	binary.BigEndian.PutUint32(sample1Size, 200)
	sample2Size := make([]byte, 4)
	binary.BigEndian.PutUint32(sample2Size, 300)

	trunPayload := append(trunHeader, sample1Size...)
	trunPayload = append(trunPayload, sample2Size...)
	trun := makeAtom("trun", trunPayload)

	var trafBytes []byte
	trafBytes = append(trafBytes, tfhd...)
	trafBytes = append(trafBytes, tfdt...)
	trafBytes = append(trafBytes, trun...)
	traf := makeAtom("traf", trafBytes)

	var moofBytes []byte
	moofBytes = append(moofBytes, mfhd...)
	moofBytes = append(moofBytes, traf...)
	moof := makeAtom("moof", moofBytes)

	// Assemble final file bytes
	fileBytes := append([]byte{}, ftyp...)
	fileBytes = append(fileBytes, moov...)
	moofOffset := len(fileBytes)
	fileBytes = append(fileBytes, moof...)
	mdat := makeAtom("mdat", make([]byte, 500))
	fileBytes = append(fileBytes, mdat...)

	// Create temp file
	tmpFile, err := os.CreateTemp("", "fmp4_test_*.mp4")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(fileBytes); err != nil {
		t.Fatalf("Failed to write mock bytes: %v", err)
	}

	// Seek back to start
	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek back: %v", err)
	}

	// Probe fMP4 using MP4Demuxer
	atoms, err := core.FastProbe(tmpFile)
	if err != nil {
		t.Fatalf("FastProbe failed: %v", err)
	}
	for _, a := range atoms {
		t.Logf("Found atom: %s, size %d, offset %d, children %d", a.Type, a.Size, a.Offset, len(a.Children))
		for _, c := range a.Children {
			t.Logf("  Child: %s, size %d, offset %d, children %d", c.Type, c.Size, c.Offset, len(c.Children))
			for _, cc := range c.Children {
				t.Logf("    Grandchild: %s, size %d, offset %d", cc.Type, cc.Size, cc.Offset)
			}
		}
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek back: %v", err)
	}

	demuxer := NewMP4Demuxer(tmpFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	t0 := tracks[0]
	if len(t0.Samples) != 2 {
		t.Fatalf("Expected 2 samples from movie fragments, got %d", len(t0.Samples))
	}

	// Verify sample 1
	s1 := t0.Samples[0]
	if s1.Time != 50 {
		t.Errorf("Expected sample 1 time 50, got %d", s1.Time)
	}
	if s1.Duration != 100 {
		t.Errorf("Expected sample 1 duration 100, got %d", s1.Duration)
	}
	if s1.Size != 200 {
		t.Errorf("Expected sample 1 size 200, got %d", s1.Size)
	}
	// offset: moofOffset + trunDataOffset (8)
	expectedOffset1 := int64(moofOffset + 8)
	if s1.Offset != expectedOffset1 {
		t.Errorf("Expected sample 1 offset %d, got %d", expectedOffset1, s1.Offset)
	}

	// Verify sample 2
	s2 := t0.Samples[1]
	if s2.Time != 150 {
		t.Errorf("Expected sample 2 time 150, got %d", s2.Time)
	}
	if s2.Duration != 100 {
		t.Errorf("Expected sample 2 duration 100, got %d", s2.Duration)
	}
	if s2.Size != 300 {
		t.Errorf("Expected sample 2 size 300, got %d", s2.Size)
	}
	expectedOffset2 := expectedOffset1 + 200
	if s2.Offset != expectedOffset2 {
		t.Errorf("Expected sample 2 offset %d, got %d", expectedOffset2, s2.Offset)
	}

	// Verify total track duration updated
	if t0.Duration != 250 {
		t.Errorf("Expected track duration 250, got %d", t0.Duration)
	}
}
