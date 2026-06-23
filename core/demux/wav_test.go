package demux

import (
	"encoding/binary"
	"io"
	"os"
	"testing"
)

func TestWAVDemuxer(t *testing.T) {
	var data []byte

	// 1. RIFF/WAVE Header
	data = append(data, 'R', 'I', 'F', 'F')
	data = append(data, 0, 0, 0, 0) // File size placeholder
	data = append(data, 'W', 'A', 'V', 'E')

	// 2. fmt chunk
	data = append(data, 'f', 'm', 't', ' ')
	
	var fmtSize [4]byte
	binary.LittleEndian.PutUint32(fmtSize[:], 16)
	data = append(data, fmtSize[:]...) // Chunk size 16

	var audioFormat [2]byte
	binary.LittleEndian.PutUint16(audioFormat[:], 1) // PCM
	data = append(data, audioFormat[:]...)

	var channels [2]byte
	binary.LittleEndian.PutUint16(channels[:], 2) // 2 channels
	data = append(data, channels[:]...)

	var sampleRate [4]byte
	binary.LittleEndian.PutUint32(sampleRate[:], 44100) // 44100Hz
	data = append(data, sampleRate[:]...)

	var byteRate [4]byte
	binary.LittleEndian.PutUint32(byteRate[:], 176400) // Byte rate
	data = append(data, byteRate[:]...)

	var blockAlign [2]byte
	binary.LittleEndian.PutUint16(blockAlign[:], 4) // 4 bytes block align
	data = append(data, blockAlign[:]...)

	var bitsPerSample [2]byte
	binary.LittleEndian.PutUint16(bitsPerSample[:], 16) // 16 bits
	data = append(data, bitsPerSample[:]...)

	// 3. data chunk
	data = append(data, 'd', 'a', 't', 'a')
	
	var dataSize [4]byte
	binary.LittleEndian.PutUint32(dataSize[:], 4096) // 1024 samples * 4 bytes/sample = 4096 bytes
	data = append(data, dataSize[:]...)

	// 4096 bytes of dummy samples
	dummySamples := make([]byte, 4096)
	for i := range dummySamples {
		dummySamples[i] = byte(i % 256)
	}
	data = append(data, dummySamples...)

	// Update RIFF size
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "wav_test_*.wav")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("Failed to write mock WAV: %v", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek back: %v", err)
	}

	demuxer := NewWAVDemuxer(tmpFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	tr := tracks[0]
	if tr.Timescale != 44100 {
		t.Errorf("Expected timescale 44100, got %d", tr.Timescale)
	}
	if tr.CodecTag != "lpcm" {
		t.Errorf("Expected CodecTag 'lpcm', got '%s'", tr.CodecTag)
	}

	// 4096 bytes / 4 blockAlign = 1024 samples, which matches 1 packet of 1024 samples exactly
	if len(tr.Samples) != 1 {
		t.Fatalf("Expected 1 sample packet, got %d", len(tr.Samples))
	}

	s := tr.Samples[0]
	if s.Size != 4096 {
		t.Errorf("Expected sample size 4096, got %d", s.Size)
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}

	if len(pkt.Data) != 4096 {
		t.Errorf("Expected packet data size 4096, got %d", len(pkt.Data))
	}
}
