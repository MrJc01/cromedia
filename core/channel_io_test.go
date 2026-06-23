package core_test

import (
	"io"
	"testing"

	"cromedia/core"
	"cromedia/core/demux"
	"cromedia/core/mux"
)

func TestChannelDemuxAndMux(t *testing.T) {
	// Create mock channels
	inputCh := make(chan *core.Packet, 5)
	outputCh := make(chan *core.Packet, 5)

	tracks := []core.Track{
		{ID: 1, Timescale: 90000},
	}

	demuxer := demux.NewChannelDemuxer(tracks, inputCh)
	muxer := mux.NewChannelMuxer(outputCh)

	// Populate input packets
	for i := 0; i < 5; i++ {
		inputCh <- &core.Packet{
			ID:          int64(i),
			StreamIndex: 0,
			Data:        []byte{byte(i)},
			PTS:         int64(i * 1000),
			DTS:         int64(i * 1000),
			Duration:    1000,
			IsKeyframe:  i == 0,
		}
	}
	close(inputCh) // Signal EOF

	// Run Demuxer -> Muxer loop
	prTracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}
	if len(prTracks) != 1 {
		t.Errorf("Expected 1 track, got %d", len(prTracks))
	}

	err = muxer.WriteHeader(prTracks)
	if err != nil {
		t.Fatalf("WriteHeader failed: %v", err)
	}

	for {
		pkt, err := demuxer.ReadPacket()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadPacket failed: %v", err)
		}

		err = muxer.WritePacket(pkt)
		if err != nil {
			t.Fatalf("WritePacket failed: %v", err)
		}
	}

	err = muxer.WriteTrailer()
	if err != nil {
		t.Fatalf("WriteTrailer failed: %v", err)
	}

	// Verify output packets
	count := 0
	for pkt := range outputCh {
		if pkt.ID != int64(count) {
			t.Errorf("Expected packet ID %d, got %d", count, pkt.ID)
		}
		if len(pkt.Data) != 1 || pkt.Data[0] != byte(count) {
			t.Errorf("Expected packet data %d, got %v", count, pkt.Data)
		}
		count++
	}

	if count != 5 {
		t.Errorf("Expected 5 packets written, got %d", count)
	}
}
