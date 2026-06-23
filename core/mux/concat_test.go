package mux

import (
	"io"
	"io/ioutil"
	"os"
	"testing"

	"cromedia/core"
	"cromedia/core/demux"
)

func TestConcatMP4Files(t *testing.T) {
	// Create track config
	tracks := []core.Track{
		{
			ID:        1,
			Type:      core.TrackTypeVideo,
			Timescale: 1000,
			Width:     640,
			Height:    480,
			CodecTag:  "avc1",
			Samples: []core.Sample{
				{ID: 1, IsKeyframe: true, Offset: 0, Size: 10, Time: 0, Duration: 500},
				{ID: 2, IsKeyframe: false, Offset: 10, Size: 15, Time: 500, Duration: 500},
			},
			Hdlr:        []byte{0, 0, 0, 0, 0, 0, 0, 0, 'v', 'i', 'd', 'e', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 'V', 'i', 'd', 'e', 'o', 0},
			MediaHeader: make([]byte, 12),
			Stsd:        make([]byte, 207), // dummy stsd of size 207
		},
	}
	// Make sure stsd is long enough
	copy(tracks[0].Stsd[12:16], "avc1")

	// Create input 1
	f1, err := ioutil.TempFile("", "concat_in1_*.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f1.Name())
	defer f1.Close()

	m1 := NewMP4Muxer(f1)
	if err := m1.WriteHeader(tracks); err != nil {
		t.Fatalf("WriteHeader 1 failed: %v", err)
	}
	if err := m1.WritePacket(&core.Packet{Data: []byte("0123456789")}); err != nil {
		t.Fatal(err)
	}
	if err := m1.WritePacket(&core.Packet{Data: []byte("ABCDEFGHIJKLMNO")}); err != nil {
		t.Fatal(err)
	}

	// Create input 2 (identical structure but with different sizes/duration)
	f2, err := ioutil.TempFile("", "concat_in2_*.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f2.Name())
	defer f2.Close()

	tracks2 := []core.Track{
		{
			ID:        1,
			Type:      core.TrackTypeVideo,
			Timescale: 1000,
			Width:     640,
			Height:    480,
			CodecTag:  "avc1",
			Samples: []core.Sample{
				{ID: 1, IsKeyframe: true, Offset: 0, Size: 5, Time: 0, Duration: 500},
			},
			Hdlr:        tracks[0].Hdlr,
			MediaHeader: tracks[0].MediaHeader,
			Stsd:        tracks[0].Stsd,
		},
	}

	m2 := NewMP4Muxer(f2)
	if err := m2.WriteHeader(tracks2); err != nil {
		t.Fatalf("WriteHeader 2 failed: %v", err)
	}
	if err := m2.WritePacket(&core.Packet{Data: []byte("XYZ12")}); err != nil {
		t.Fatal(err)
	}

	// Output file
	outFile, err := ioutil.TempFile("", "concat_out_*.mp4")
	if err != nil {
		t.Fatal(err)
	}
	outPath := outFile.Name()
	outFile.Close()
	defer os.Remove(outPath)

	// Concat
	err = ConcatMP4Files(outPath, []string{f1.Name(), f2.Name()})
	if err != nil {
		t.Fatalf("ConcatMP4Files failed: %v", err)
	}

	// Demux & Verify
	verifyF, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer verifyF.Close()

	d := demux.NewMP4Demuxer(verifyF)
	outTracks, err := d.Probe()
	if err != nil {
		t.Fatalf("Probe of concatenated file failed: %v", err)
	}

	if len(outTracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(outTracks))
	}

	ot := outTracks[0]
	if len(ot.Samples) != 3 {
		t.Errorf("Expected 3 samples, got %d", len(ot.Samples))
	}

	// Verify durations and timestamps
	expectedTimes := []int64{0, 500, 1000}
	for i, s := range ot.Samples {
		if s.Time != expectedTimes[i] {
			t.Errorf("Sample %d has Time %d, expected %d", i, s.Time, expectedTimes[i])
		}
	}

	// Read packets and verify payload
	var dataChunks []string
	for {
		pkt, err := d.ReadPacket()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		dataChunks = append(dataChunks, string(pkt.Data))
	}

	if len(dataChunks) != 3 {
		t.Fatalf("Expected 3 packets read, got %d", len(dataChunks))
	}
	if dataChunks[0] != "0123456789" {
		t.Errorf("Packet 0 payload mismatch: got %q", dataChunks[0])
	}
	if dataChunks[1] != "ABCDEFGHIJKLMNO" {
		t.Errorf("Packet 1 payload mismatch: got %q", dataChunks[1])
	}
	if dataChunks[2] != "XYZ12" {
		t.Errorf("Packet 2 payload mismatch: got %q", dataChunks[2])
	}
}
