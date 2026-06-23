package core

import (
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

func TestFastProbe(t *testing.T) {
	// Create a temporary file mimicking an MP4
	tmpfile, err := ioutil.TempFile("", "example.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up

	// Writes an atom to the file
	writeAtom := func(typ string, size uint32) {
		b := make([]byte, 8)
		binary.BigEndian.PutUint32(b[0:4], size)
		copy(b[4:8], []byte(typ))
		tmpfile.Write(b)
	}

	// Write 'ftyp' atom (size 20)
	writeAtom("ftyp", 20)
	tmpfile.Write(make([]byte, 12)) // payload

	// Write 'moov' atom (container)
	// We'll calculate size later or just hardcode for this simple test
	// moov header (8) + mvhd (100) = 108
	writeAtom("moov", 108)

	// Write 'mvhd' inside 'moov'
	writeAtom("mvhd", 100)
	tmpfile.Write(make([]byte, 92)) // payload

	// Write 'mdat' (size 1000)
	writeAtom("mdat", 1000)
	tmpfile.Write(make([]byte, 992))

	tmpfile.Sync()
	tmpfile.Seek(0, 0)

	// Test Probing
	atoms, err := FastProbe(tmpfile)
	if err != nil {
		t.Fatalf("FastProbe failed: %v", err)
	}

	if len(atoms) != 3 {
		t.Errorf("Expected 3 top-level atoms, got %d", len(atoms))
	}

	if atoms[0].Type != "ftyp" {
		t.Errorf("Expected first atom to be ftyp, got %s", atoms[0].Type)
	}

	if atoms[1].Type != "moov" {
		t.Errorf("Expected second atom to be moov, got %s", atoms[1].Type)
	}

	if len(atoms[1].Children) != 1 {
		t.Errorf("Expected moov to have 1 child, got %d", len(atoms[1].Children))
	}

	if atoms[1].Children[0].Type != "mvhd" {
		t.Errorf("Expected child of moov to be mvhd, got %s", atoms[1].Children[0].Type)
	}
}

func TestGetMP4Duration(t *testing.T) {
	// 1. Test Version 0 mvhd
	tmpfile, err := ioutil.TempFile("", "duration_v0.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	writeAtom := func(file *os.File, typ string, size uint32) {
		b := make([]byte, 8)
		binary.BigEndian.PutUint32(b[0:4], size)
		copy(b[4:8], []byte(typ))
		file.Write(b)
	}

	// moov (container): size = header (8) + mvhd (108) = 116
	writeAtom(tmpfile, "moov", 116)
	
	// mvhd: size = 108
	writeAtom(tmpfile, "mvhd", 108)
	
	// mvhd payload: version (1 byte) = 0 + flags (3 bytes) = 0
	// creation time (4 bytes) = 0, modification time (4 bytes) = 0
	// timescale (4 bytes) = 1000 (0x000003e8), duration (4 bytes) = 5500 (0x0000157c)
	// then 80 bytes padding
	payload := make([]byte, 100)
	payload[0] = 0 // version 0
	binary.BigEndian.PutUint32(payload[12:16], 1000) // timescale
	binary.BigEndian.PutUint32(payload[16:20], 5500) // duration
	tmpfile.Write(payload)

	tmpfile.Sync()
	tmpfile.Seek(0, io.SeekStart)

	dur, err := GetMP4Duration(tmpfile)
	if err != nil {
		t.Fatalf("Failed to get duration: %v", err)
	}
	if dur != 5500 {
		t.Errorf("Expected duration 5500, got %d", dur)
	}

	// 2. Test Version 1 mvhd
	tmpfile2, err := ioutil.TempFile("", "duration_v1.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile2.Name())

	// moov (container): size = header (8) + mvhd (120) = 128
	writeAtom(tmpfile2, "moov", 128)
	
	// mvhd: size = 120
	writeAtom(tmpfile2, "mvhd", 120)
	
	// mvhd payload: version (1 byte) = 1 + flags (3 bytes) = 0
	// creation time (8 bytes) = 0, modification time (8 bytes) = 0
	// timescale (4 bytes) = 1000 (0x000003e8), duration (8 bytes) = 9900 (0x00000000000026ac)
	// then 92 bytes padding
	payload2 := make([]byte, 112)
	payload2[0] = 1 // version 1
	binary.BigEndian.PutUint32(payload2[20:24], 1000) // timescale
	binary.BigEndian.PutUint64(payload2[24:32], 9900) // duration
	tmpfile2.Write(payload2)

	tmpfile2.Sync()
	tmpfile2.Seek(0, io.SeekStart)

	dur2, err := GetMP4Duration(tmpfile2)
	if err != nil {
		t.Fatalf("Failed to get duration version 1: %v", err)
	}
	if dur2 != 9900 {
		t.Errorf("Expected duration 9900, got %d", dur2)
	}
}

