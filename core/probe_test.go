package core

import (
	"encoding/binary"
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
