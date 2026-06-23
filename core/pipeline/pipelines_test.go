package pipeline

import (
	"encoding/base64"
	"os"
	"testing"

	"cromedia/core"
	"cromedia/core/demux"
	"cromedia/core/mux"
)

// Helper to create a mock MP4 file with video and optionally audio track
func createMockMP4(t *testing.T, filename string, includeAudio bool) {
	t.Helper()

	videoTrack := core.Track{
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
		Hdlr:        mux.DefaultVideoHdlr(),
		MediaHeader: mux.DefaultVideoMediaHeader(),
		Stsd:        make([]byte, 207),
	}
	copy(videoTrack.Stsd[12:16], "avc1")

	var tracks []core.Track
	tracks = append(tracks, videoTrack)

	if includeAudio {
		audioTrack := core.Track{
			ID:        2,
			Type:      core.TrackTypeAudio,
			Timescale: 44100,
			CodecTag:  "mp4a",
			Samples: []core.Sample{
				{ID: 1, IsKeyframe: true, Offset: 25, Size: 8, Time: 0, Duration: 1024},
			},
			Hdlr:        mux.DefaultAudioHdlr(),
			MediaHeader: mux.DefaultAudioMediaHeader(),
			Stsd:        make([]byte, 100),
		}
		copy(audioTrack.Stsd[12:16], "mp4a")
		tracks = append(tracks, audioTrack)
	}

	f, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	m := mux.NewMP4Muxer(f)
	if err := m.WriteHeader(tracks); err != nil {
		t.Fatalf("WriteHeader failed: %v", err)
	}

	// Write video packets payload
	if err := m.WritePacket(&core.Packet{Data: []byte("0123456789")}); err != nil {
		t.Fatal(err)
	}
	if err := m.WritePacket(&core.Packet{Data: []byte("ABCDEFGHIJKLMNO")}); err != nil {
		t.Fatal(err)
	}

	if includeAudio {
		// Write audio packets payload
		if err := m.WritePacket(&core.Packet{Data: []byte("VOICE123")}); err != nil {
			t.Fatal(err)
		}
	}
}

// Helper to create a mock valid MP3 file
func createMockMP3(t *testing.T, filename string) {
	t.Helper()

	b64 := "SUQzBAAAAAAAI1RTU0UAAAAPAAADTGF2ZjU2LjM2LjEwMAAAAAAAAAAAAAAA//OEAAAAAAAAAAAAAAAAAAAAAAAASW5mbwAAAA8AAAAEAAABIADAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDV1dXV1dXV1dXV1dXV1dXV1dXV1dXV1dXV6urq6urq6urq6urq6urq6urq6urq6urq6v////////////////////////////////8AAAAATGF2YzU2LjQxAAAAAAAAAAAAAAAAJAAAAAAAAAAAASDs90hvAAAAAAAAAAAAAAAAAAAA//MUZAAAAAGkAAAAAAAAA0gAAAAATEFN//MUZAMAAAGkAAAAAAAAA0gAAAAARTMu//MUZAYAAAGkAAAAAAAAA0gAAAAAOTku//MUZAkAAAGkAAAAAAAAA0gAAAAANVVV"
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
}

func TestOptimizeVideoAsset(t *testing.T) {
	inputMP4 := "test_optimize_input.mp4"
	outputMP4 := "test_optimize_output.mp4"

	createMockMP4(t, inputMP4, true)
	defer os.Remove(inputMP4)
	defer os.Remove(outputMP4)

	err := OptimizeVideoAsset(inputMP4, outputMP4)
	if err != nil {
		t.Fatalf("OptimizeVideoAsset failed: %v", err)
	}

	// Verify output
	f, err := os.Open(outputMP4)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	d := demux.NewMP4Demuxer(f)
	tracks, err := d.Probe()
	if err != nil {
		t.Fatalf("Probe on optimized output failed: %v", err)
	}

	if len(tracks) != 2 {
		t.Fatalf("Expected 2 tracks (video + audio), got %d", len(tracks))
	}

	if tracks[0].Width != 1920 || tracks[0].Height != 1080 {
		t.Errorf("Expected optimized width/height to be 1920x1080, got %dx%d", tracks[0].Width, tracks[0].Height)
	}
}

func TestMixAudioTracks(t *testing.T) {
	videoMP4 := "test_mix_video.mp4"
	soundtrackMP3 := "test_mix_soundtrack.mp3"
	outputMP4 := "test_mix_output.mp4"

	createMockMP4(t, videoMP4, true)
	createMockMP3(t, soundtrackMP3)
	defer os.Remove(videoMP4)
	defer os.Remove(soundtrackMP3)
	defer os.Remove(outputMP4)

	err := MixAudioTracks(videoMP4, soundtrackMP3, outputMP4)
	if err != nil {
		t.Fatalf("MixAudioTracks failed: %v", err)
	}

	// Verify output
	f, err := os.Open(outputMP4)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	d := demux.NewMP4Demuxer(f)
	tracks, err := d.Probe()
	if err != nil {
		t.Fatalf("Probe on mixed output failed: %v", err)
	}

	if len(tracks) != 2 {
		t.Fatalf("Expected 2 tracks, got %d", len(tracks))
	}

	if tracks[1].Type != core.TrackTypeAudio {
		t.Errorf("Expected track 1 to be audio, got %s", tracks[1].Type)
	}
}
