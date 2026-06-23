package demux

import (
	"io"
	"os"
	"testing"
)

func makeTSPacket(pid uint16, pusi bool, af []byte, payload []byte) []byte {
	buf := make([]byte, 188)
	buf[0] = 0x47

	var pusiByte byte = 0
	if pusi {
		pusiByte = 0x40
	}
	buf[1] = pusiByte | byte(pid>>8)
	buf[2] = byte(pid)

	if len(af) > 0 {
		buf[3] = 0xC0 // AFC = 3, CC = 0
		buf[4] = byte(len(af))
		copy(buf[5:5+len(af)], af)
		payloadStart := 5 + len(af)
		copy(buf[payloadStart:], payload)
		for i := payloadStart + len(payload); i < 188; i++ {
			buf[i] = 0xFF
		}
	} else {
		buf[3] = 0x40 // AFC = 1, CC = 0
		copy(buf[4:], payload)
		for i := 4 + len(payload); i < 188; i++ {
			buf[i] = 0xFF
		}
	}
	return buf
}

func TestTSDemuxer(t *testing.T) {
	// 1. Construct PAT Packet (PID = 0, PUSI = true)
	patPayload := []byte{
		0x00,       // pointer field
		0x00,       // table_id
		0xB0, 0x0D, // section length
		0x00, 0x01, // transport_stream_id
		0xC1, // version/indicator
		0x00, // section_number
		0x00, // last_section_number
		0x00, 0x01, // program_number
		0xE1, 0x00, // program_map_PID (256)
		0x00, 0x00, 0x00, 0x00, // CRC
	}
	patPacket := makeTSPacket(0, true, nil, patPayload)

	// 2. Construct PMT Packet (PID = 256, PUSI = true)
	pmtPayload := []byte{
		0x00,       // pointer field
		0x02,       // table_id
		0xB0, 0x17, // section length
		0x00, 0x01, // program_number
		0xC1, // version/indicator
		0x00, // section_number
		0x00, // last_section_number
		0xE1, 0x00, // PCR PID (256)
		0xF0, 0x00, // program info length
		0x1B,       // H.264 video
		0xE1, 0x01, // video PID (257)
		0xF0, 0x00, // ES info length
		0x0F,       // AAC audio
		0xE1, 0x02, // audio PID (258)
		0xF0, 0x00, // ES info length
		0x00, 0x00, 0x00, 0x00, // CRC
	}
	pmtPacket := makeTSPacket(256, true, nil, pmtPayload)

	// 3. Construct PES Video Packet (PID = 257, PUSI = true)
	// PTS = 90000 (1 second at 90kHz timescale)
	pesVideoPayload := []byte{
		0x00, 0x00, 0x01, // prefix
		0xE0,       // streamID
		0x00, 0x1A, // PES length (26 bytes)
		0x80, 0x80, // flags
		0x05,                         // header data length
		0x21, 0x00, 0x05, 0xBF, 0x21, // PTS 90000 (last byte corrected to 0x21)
		0x00, 0x00, 0x00, 0x01, 0x65, // H.264 start code + NAL type 5 (IDR keyframe)
		'v', 'i', 'd', 'e', 'o', 'd', 'a', 't', 'a',
	}
	// Adaptation field of 155 bytes to prevent packet pollution
	videoPacket := makeTSPacket(257, true, make([]byte, 155), pesVideoPayload)

	// Assemble final file bytes
	var fileBytes []byte
	fileBytes = append(fileBytes, patPacket...)
	fileBytes = append(fileBytes, pmtPacket...)
	videoOffset := len(fileBytes)
	fileBytes = append(fileBytes, videoPacket...)

	// Create temp file
	tmpFile, err := os.CreateTemp("", "ts_test_*.ts")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(fileBytes); err != nil {
		t.Fatalf("Failed to write mock TS: %v", err)
	}

	// Seek back to start
	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek back: %v", err)
	}

	// Probe using TSDemuxer
	demuxer := NewTSDemuxer(tmpFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	// Verify we parsed 2 tracks mapped in the PMT
	if len(tracks) != 2 {
		t.Fatalf("Expected 2 tracks (video + audio), got %d", len(tracks))
	}

	vTrack := tracks[0]
	if vTrack.ID != 257 {
		t.Errorf("Expected video track ID 257, got %d", vTrack.ID)
	}
	if vTrack.CodecTag != "avc1" {
		t.Errorf("Expected video codec tag 'avc1', got '%s'", vTrack.CodecTag)
	}
	if vTrack.Timescale != 90000 {
		t.Errorf("Expected video timescale 90000, got %d", vTrack.Timescale)
	}

	// Verify that the video sample was extracted and mapped correctly
	if len(vTrack.Samples) != 1 {
		t.Fatalf("Expected 1 video sample, got %d", len(vTrack.Samples))
	}

	s := vTrack.Samples[0]
	if s.Time != 90000 {
		t.Errorf("Expected sample time (PTS) 90000, got %d", s.Time)
	}
	if !s.IsKeyframe {
		t.Errorf("Expected sample to be detected as keyframe based on NAL type 5")
	}
	// size of payload: total len(pesVideoPayload) - headerLen (14) = 28 - 14 = 14 bytes
	if s.Size != 14 {
		t.Errorf("Expected sample size 14, got %d", s.Size)
	}
	// offset: videoOffset + header (4) + af_len_indicator (1) + af (155) + PESHeader (14) = videoOffset + 174
	expectedOffset := int64(videoOffset + 174)
	if s.Offset != expectedOffset {
		t.Errorf("Expected sample offset %d, got %d", expectedOffset, s.Offset)
	}

	// Read packet
	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}

	// Check packet payload matches media bytes (H.264 start code + NAL + "videodata")
	expectedData := []byte{0x00, 0x00, 0x00, 0x01, 0x65, 'v', 'i', 'd', 'e', 'o', 'd', 'a', 't', 'a'}
	if string(pkt.Data) != string(expectedData) {
		t.Errorf("Expected packet data %v, got %v", expectedData, pkt.Data)
	}

	if pkt.PTS != 90000 {
		t.Errorf("Expected packet PTS 90000, got %d", pkt.PTS)
	}

	// Next read should EOF
	_, err = demuxer.ReadPacket()
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got %v", err)
	}
}
