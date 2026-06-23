package mux

import (
	"os"
	"testing"

	"cromedia/core"
)

func TestMuxers(t *testing.T) {
	tracks := []core.Track{
		{
			ID:        1,
			Type:      core.TrackTypeVideo,
			Timescale: 90000,
			Width:     1920,
			Height:    1080,
			CodecTag:  "avc1",
		},
		{
			ID:        2,
			Type:      core.TrackTypeAudio,
			Timescale: 44100,
			CodecTag:  "mp4a",
		},
	}

	pktVideo := &core.Packet{
		StreamIndex: 0,
		PTS:         0,
		DTS:         0,
		Duration:    3000,
		IsKeyframe:  true,
		Data:        []byte{0, 0, 0, 1, 0x67, 0x42, 0, 0x1e, 0x9d, 0xa0, 0x50, 0x17, 0xfc, 0xb8, 0x0a, 0x90, 0x71, 0x01, 0x11, 0, 0, 0, 1, 0x68, 0xce, 0x3c, 0x80},
	}

	pktAudio := &core.Packet{
		StreamIndex: 1,
		PTS:         0,
		DTS:         0,
		Duration:    1024,
		Data:        []byte{0x11, 0x90},
	}

	// 1. WAV Muxer
	t.Run("WAV", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.wav")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewWAVMuxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(pktAudio); err != nil {
			t.Fatal(err)
		}
		if err := m.WriteTrailer(); err != nil {
			t.Fatal(err)
		}
	})

	// 2. MP3 Muxer
	t.Run("MP3", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.mp3")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewMP3Muxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(pktAudio); err != nil {
			t.Fatal(err)
		}
	})

	// 3. FLAC Muxer
	t.Run("FLAC", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.flac")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewFLACMuxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(pktAudio); err != nil {
			t.Fatal(err)
		}
	})

	// 4. AAC Muxer
	t.Run("AAC", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.aac")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewAACMuxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(pktAudio); err != nil {
			t.Fatal(err)
		}
	})

	// 5. Ogg Muxer
	t.Run("Ogg", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.ogg")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewOggMuxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(pktAudio); err != nil {
			t.Fatal(err)
		}
	})

	// 6. FLV Muxer
	t.Run("FLV", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.flv")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewFLVMuxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(pktVideo); err != nil {
			t.Fatal(err)
		}
	})

	// 7. fMP4 Muxer
	t.Run("fMP4", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.mp4")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewFMP4Muxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(pktVideo); err != nil {
			t.Fatal(err)
		}
	})

	// 8. WebM Muxer
	t.Run("WebM", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.webm")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewWebMMuxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(pktVideo); err != nil {
			t.Fatal(err)
		}
	})

	// 9. MKV Muxer
	t.Run("MKV", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.mkv")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewMKVMuxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(pktVideo); err != nil {
			t.Fatal(err)
		}
	})

	// 10. TS Muxer
	t.Run("TS", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.ts")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewTSMuxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(pktVideo); err != nil {
			t.Fatal(err)
		}
	})

	// 11. AnnexB Muxer
	t.Run("AnnexB", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.264")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewAnnexBMuxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(pktVideo); err != nil {
			t.Fatal(err)
		}
	})

	// 12. SRT Muxer
	t.Run("SRT", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.srt")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewSRTMuxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(&core.Packet{Data: []byte("Subtitle text"), PTS: 1000, Duration: 2000}); err != nil {
			t.Fatal(err)
		}
	})

	// 13. WebVTT Muxer
	t.Run("WebVTT", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_*.vtt")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		m := NewWebVTTMuxer(f)
		if err := m.WriteHeader(tracks); err != nil {
			t.Fatal(err)
		}
		if err := m.WritePacket(&core.Packet{Data: []byte("WebVTT text"), PTS: 1000, Duration: 2000}); err != nil {
			t.Fatal(err)
		}
	})

	// 14. Disk Full Simulation (error propagation check)
	t.Run("DiskFullSimulation", func(t *testing.T) {
		f, err := os.CreateTemp("", "test_disk_full_*.wav")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		
		m := NewWAVMuxer(f)
		
		// Close the file descriptor immediately to trigger write failure (simulating disk full / bad file descriptor)
		f.Close()
		
		err = m.WriteHeader(tracks)
		if err == nil {
			t.Error("expected write error on closed file, got nil")
		}
	})
}
