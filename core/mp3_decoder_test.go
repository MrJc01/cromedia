package core

import (
	"testing"
)

func TestSimMP3DecoderBasic(t *testing.T) {
	dec := &SimMP3Decoder{}
	defer dec.Close()

	frame, err := dec.Decode(nil)
	if err != nil {
		t.Fatalf("Decode(nil) failed: %v", err)
	}
	if frame != nil {
		t.Fatalf("Expected nil frame, got %v", frame)
	}
}
