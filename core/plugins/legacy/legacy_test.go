//go:build legacy

package legacy

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"cromedia/core"
)

func createFakeAVI() []byte {
	buf := new(bytes.Buffer)
	buf.Write([]byte("RIFF"))
	binary.Write(buf, binary.LittleEndian, uint32(250)) // size placeholder
	buf.Write([]byte("AVI "))
	
	// hdrl LIST
	buf.Write([]byte("LIST"))
	binary.Write(buf, binary.LittleEndian, uint32(116))
	buf.Write([]byte("hdrl"))
	
	// avih
	buf.Write([]byte("avih"))
	binary.Write(buf, binary.LittleEndian, uint32(56))
	buf.Write(make([]byte, 56))
	
	// strl LIST
	buf.Write([]byte("LIST"))
	binary.Write(buf, binary.LittleEndian, uint32(40))
	buf.Write([]byte("strl"))
	
	// strh
	buf.Write([]byte("strh"))
	binary.Write(buf, binary.LittleEndian, uint32(48))
	buf.Write([]byte("vids"))         // 0:4
	buf.Write([]byte("DIB "))         // 4:8
	buf.Write([]byte("DIB "))         // 8:12 (Codec)
	buf.Write(make([]byte, 8))        // 12:20
	binary.Write(buf, binary.LittleEndian, uint32(1))  // 20:24 (scale)
	binary.Write(buf, binary.LittleEndian, uint32(25)) // 24:28 (rate)
	buf.Write(make([]byte, 20))       // 28:48

	
	// movi LIST
	buf.Write([]byte("LIST"))
	binary.Write(buf, binary.LittleEndian, uint32(16))
	buf.Write([]byte("movi"))
	
	// frame 0 (00db)
	buf.Write([]byte("00db"))
	binary.Write(buf, binary.LittleEndian, uint32(4))
	buf.Write([]byte("AAAA"))
	
	// idx1
	buf.Write([]byte("idx1"))
	binary.Write(buf, binary.LittleEndian, uint32(16))
	buf.Write([]byte("00db"))
	binary.Write(buf, binary.LittleEndian, uint32(0x10)) // AVIIF_KEYFRAME flag
	binary.Write(buf, binary.LittleEndian, uint32(0)) // offset relative to moviPos
	binary.Write(buf, binary.LittleEndian, uint32(4)) // size
	
	return buf.Bytes()
}

func TestAVIDemuxer(t *testing.T) {
	data := createFakeAVI()
	tmpFile, err := os.CreateTemp("", "test_avi_*.avi")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatal(err)
	}

	_, _ = tmpFile.Seek(0, 0)
	demuxer, err := NewAVIDemuxer(tmpFile)
	if err != nil {
		t.Fatalf("failed to create AVI demuxer: %v", err)
	}
	defer demuxer.Close()

	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("failed to probe tracks: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if tracks[0].CodecTag != "DIB " {
		t.Errorf("expected DIB codec, got %q", tracks[0].CodecTag)
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("failed to read packet: %v", err)
	}
	if string(pkt.Data) != "AAAA" {
		t.Errorf("expected AAAA packet data, got %q", string(pkt.Data))
	}

	// Test Seeking
	err = demuxer.Seek(0)
	if err != nil {
		t.Errorf("Seek failed: %v", err)
	}
}

func createFakeASF() []byte {
	buf := new(bytes.Buffer)
	buf.Write(ASFHeaderObjectGUID[:])
	binary.Write(buf, binary.LittleEndian, uint64(200)) // size
	binary.Write(buf, binary.LittleEndian, uint32(2)) // 2 sub-objects
	buf.Write(make([]byte, 2)) // reserved

	// Sub-object 1: File Properties
	buf.Write(ASFFileProperties[:])
	binary.Write(buf, binary.LittleEndian, uint64(88))
	filePropData := make([]byte, 64)
	binary.LittleEndian.PutUint32(filePropData[56:60], uint32(50)) // packetSize = 50
	buf.Write(filePropData)

	// Sub-object 2: Stream Properties
	buf.Write(ASFStreamProperties[:])
	binary.Write(buf, binary.LittleEndian, uint64(78))
	// streamType starts with 0x81, 0xEF for video
	streamType := GUID{0x81, 0xEF}
	buf.Write(streamType[:])
	buf.Write(make([]byte, 16)) // dummy error correction type GUID
	binary.Write(buf, binary.LittleEndian, uint64(0)) // timeOffset
	binary.Write(buf, binary.LittleEndian, uint32(0)) // type-specific data size
	binary.Write(buf, binary.LittleEndian, uint32(0)) // error correction data size
	binary.Write(buf, binary.LittleEndian, uint16(1)) // flags (stream ID = 1)
	buf.Write(make([]byte, 4)) // dummy type-specific remaining bytes to pad to size

	// Data Object
	buf.Write(ASFDataObjectGUID[:])
	binary.Write(buf, binary.LittleEndian, uint64(126)) // data object size
	buf.Write(make([]byte, 16)) // Skip file ID + total packets
	
	// Add 2 dummy packets of 50 bytes each
	buf.Write(make([]byte, 50))
	buf.Write(make([]byte, 50))

	return buf.Bytes()
}

func TestASFDemuxer(t *testing.T) {
	data := createFakeASF()
	tmpFile, err := os.CreateTemp("", "test_asf_*.asf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatal(err)
	}

	_, _ = tmpFile.Seek(0, 0)
	demuxer, err := NewASFDemuxer(tmpFile)
	if err != nil {
		t.Fatalf("failed to create ASF demuxer: %v", err)
	}
	defer demuxer.Close()

	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("failed to probe tracks: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if tracks[0].CodecTag != "wmv" {
		t.Errorf("expected wmv codec, got %q", tracks[0].CodecTag)
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("failed to read packet: %v", err)
	}
	if len(pkt.Data) != 50 {
		t.Errorf("expected packet size 50, got %d", len(pkt.Data))
	}
}

func createFakeRM() []byte {
	buf := new(bytes.Buffer)
	buf.Write([]byte(".RMF"))
	binary.Write(buf, binary.BigEndian, uint32(0)) // size
	binary.Write(buf, binary.BigEndian, uint16(0)) // version
	binary.Write(buf, binary.BigEndian, uint32(2)) // num headers

	// MDPR chunk
	buf.Write([]byte("MDPR"))
	binary.Write(buf, binary.BigEndian, uint32(66)) // chunk size (was 80)
	binary.Write(buf, binary.BigEndian, uint16(0)) // version
	binary.Write(buf, binary.BigEndian, uint16(1)) // stream ID = 1
	buf.Write(make([]byte, 28)) // bits, rates, times, etc.
	buf.Write([]byte{4}) // streamName length
	buf.Write([]byte("test"))
	buf.Write([]byte{20}) // mimeType length
	buf.Write([]byte("video/x-pn-realvideo"))

	// DATA chunk
	buf.Write([]byte("DATA"))
	binary.Write(buf, binary.BigEndian, uint32(38)) // chunk size (was 50)
	binary.Write(buf, binary.BigEndian, uint16(0)) // version
	binary.Write(buf, binary.BigEndian, uint32(1)) // num packets

	// Packet data
	binary.Write(buf, binary.BigEndian, uint16(0)) // version
	binary.Write(buf, binary.BigEndian, uint16(24)) // length of packet
	binary.Write(buf, binary.BigEndian, uint16(1)) // stream ID = 1
	binary.Write(buf, binary.BigEndian, uint32(100)) // timestamp = 100 ms
	buf.Write(make([]byte, 2)) // flags/skip
	buf.Write([]byte("123456789012")) // 12 bytes payload data

	return buf.Bytes()
}

func TestRealMediaDemuxer(t *testing.T) {
	data := createFakeRM()
	tmpFile, err := os.CreateTemp("", "test_rm_*.rm")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatal(err)
	}

	_, _ = tmpFile.Seek(0, 0)
	demuxer, err := NewRealMediaDemuxer(tmpFile)
	if err != nil {
		t.Fatalf("failed to create RealMedia demuxer: %v", err)
	}
	defer demuxer.Close()

	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("failed to probe tracks: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if tracks[0].CodecTag != "rv40" {
		t.Errorf("expected rv40, got %q", tracks[0].CodecTag)
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("failed to read packet: %v", err)
	}
	if string(pkt.Data) != "123456789012" {
		t.Errorf("expected payload data, got %q", string(pkt.Data))
	}
}

func createFakeMP2() []byte {
	buf := new(bytes.Buffer)
	// Frame 1
	// Sync: 0xFF + 0xFD (MPEG-1 Layer II: sync + layer bits)
	// Bitrate index: 8 (128 kbps), sample rate index: 0 (44.1 kHz), padding: 0
	// 144 * 128000 / 44100 = 417 bytes size
	buf.Write([]byte{0xFF, 0xFD, 0x80, 0x00})
	buf.Write(make([]byte, 413))

	return buf.Bytes()
}

func TestMP2DemuxerAndDecoder(t *testing.T) {
	data := createFakeMP2()
	tmpFile, err := os.CreateTemp("", "test_mp2_*.mp2")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatal(err)
	}

	_, _ = tmpFile.Seek(0, 0)
	demuxer, err := NewMP2Demuxer(tmpFile)
	if err != nil {
		t.Fatalf("failed to create MP2 demuxer: %v", err)
	}
	defer demuxer.Close()

	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("failed to probe tracks: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if tracks[0].CodecTag != "mp2" {
		t.Errorf("expected mp2 codec, got %q", tracks[0].CodecTag)
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("failed to read packet: %v", err)
	}
	if len(pkt.Data) != 417 {
		t.Errorf("expected frame size 417, got %d", len(pkt.Data))
	}

	// Test Decoder
	dec := &MP2Decoder{}
	defer dec.Close()

	frame, err := dec.Decode(pkt)
	if err != nil {
		t.Fatalf("decoder failed: %v", err)
	}
	if frame.SampleRate != 44100 {
		t.Errorf("expected 44100 Hz, got %d", frame.SampleRate)
	}
}

func TestDecoders(t *testing.T) {
	pkt := &core.Packet{
		Data: make([]byte, 100),
	}

	wmv := &WMVDecoder{}
	if f, err := wmv.Decode(pkt); err != nil || f.Width != 320 {
		t.Errorf("WMVDecoder test failed")
	}

	wma := &WMADecoder{}
	if f, err := wma.Decode(pkt); err != nil || f.SampleRate != 44100 {
		t.Errorf("WMADecoder test failed")
	}

	sorenson := &SorensonDecoder{}
	if f, err := sorenson.Decode(pkt); err != nil || f.Width != 320 {
		t.Errorf("SorensonDecoder test failed")
	}

	adpcm := &ADPCMDecoder{}
	if f, err := adpcm.Decode(pkt); err != nil || f.SampleRate != 11025 {
		t.Errorf("ADPCMDecoder test failed")
	}

	amr := &AMRDecoder{}
	if f, err := amr.Decode(pkt); err != nil || f.SampleRate != 8000 {
		t.Errorf("AMRDecoder test failed")
	}
}
