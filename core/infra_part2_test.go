package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// DummyCodec implements Codec interface.
type DummyCodec struct {
	name string
}

func (d *DummyCodec) Name() string     { return d.name }
func (d *DummyCodec) Type() TrackType { return TrackTypeVideo }

func TestCodecRegistry(t *testing.T) {
	meta := Codec{
		Name:        "mock_codec",
		Type:        TrackTypeVideo,
		Description: "Mock Codec for Testing",
	}

	decFactory := func() (interface{}, error) { return "decoder", nil }
	encFactory := func() (interface{}, error) { return "encoder", nil }

	RegisterCodec(meta, decFactory, encFactory)

	// Check registry retrieval
	dec, err := GetDecoder("mock_codec")
	if err != nil {
		t.Fatalf("Expected decoder registered, got error: %v", err)
	}
	decVal, _ := dec()
	if decVal.(string) != "decoder" {
		t.Errorf("Expected 'decoder', got %v", decVal)
	}

	enc, err := GetEncoder("mock_codec")
	if err != nil {
		t.Fatalf("Expected encoder registered, got error: %v", err)
	}
	encVal, _ := enc()
	if encVal.(string) != "encoder" {
		t.Errorf("Expected 'encoder', got %v", encVal)
	}

	// Verify List
	list := ListCodecs()
	found := false
	for _, c := range list {
		if c.Name == "mock_codec" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'mock_codec' in listed codecs")
	}

	// Negative check
	_, err = GetDecoder("non_existent")
	if err == nil {
		t.Error("Expected error fetching non-existent decoder")
	}
}

func TestCFRFilter(t *testing.T) {
	// FPS: 10, Timescale: 1000 => 100ms per frame
	filter := NewCFRFilter(10.0, 1000)

	// Packets with timing: 0, 100, 200, 500
	p1 := &Packet{ID: 1, StreamIndex: 0, PTS: 0, DTS: 0, Data: []byte("a")}
	p2 := &Packet{ID: 2, StreamIndex: 0, PTS: 100, DTS: 100, Data: []byte("b")}
	p3 := &Packet{ID: 3, StreamIndex: 0, PTS: 200, DTS: 200, Data: []byte("c")}
	p4 := &Packet{ID: 4, StreamIndex: 0, PTS: 500, DTS: 500, Data: []byte("d")}

	var out []*Packet
	out = append(out, filter.Process(p1)...)
	out = append(out, filter.Process(p2)...)
	out = append(out, filter.Process(p3)...)
	out = append(out, filter.Process(p4)...)

	// We expect:
	// p1 at PTS 0
	// p2 at PTS 100
	// p3 at PTS 200
	// duplicated p3 at PTS 300, 400
	// p4 at PTS 500
	expectedPTS := []int64{0, 100, 200, 300, 400, 500}
	if len(out) != len(expectedPTS) {
		t.Fatalf("Expected %d output packets, got %d", len(expectedPTS), len(out))
	}

	for i, p := range out {
		if p.PTS != expectedPTS[i] {
			t.Errorf("Packet %d: Expected PTS %d, got %d", i, expectedPTS[i], p.PTS)
		}
		if p.Duration != 100 {
			t.Errorf("Packet %d: Expected Duration 100, got %d", i, p.Duration)
		}
	}
}

func TestVFRFilter(t *testing.T) {
	filter := NewVFRFilter()

	p1 := &Packet{ID: 1, PTS: 10, DTS: 10}
	p2 := &Packet{ID: 2, PTS: 10, DTS: 10} // Duplicate PTS
	p3 := &Packet{ID: 3, PTS: 5, DTS: 5}   // Out of order/backwards PTS
	p4 := &Packet{ID: 4, PTS: 20, DTS: 20}

	o1 := filter.Process(p1)
	o2 := filter.Process(p2)
	o3 := filter.Process(p3)
	o4 := filter.Process(p4)

	if o1.PTS != 10 {
		t.Errorf("Expected 10, got %d", o1.PTS)
	}
	if o2.PTS != 11 {
		t.Errorf("Expected VFR normalisation to 11, got %d", o2.PTS)
	}
	if o3.PTS != 12 {
		t.Errorf("Expected VFR normalisation to 12, got %d", o3.PTS)
	}
	if o4.PTS != 20 {
		t.Errorf("Expected VFR 20, got %d", o4.PTS)
	}
}

func TestSyncBarrier(t *testing.T) {
	ch1 := make(chan *Packet, 5)
	ch2 := make(chan *Packet, 5)

	barrier := NewSyncBarrier([]<-chan *Packet{ch1, ch2})

	ch1 <- &Packet{ID: 1, PTS: 10}
	ch1 <- &Packet{ID: 3, PTS: 30}
	ch1 <- &Packet{ID: 5, PTS: 50}
	close(ch1)

	ch2 <- &Packet{ID: 2, PTS: 20}
	ch2 <- &Packet{ID: 4, PTS: 40}
	ch2 <- &Packet{ID: 6, PTS: 60}
	close(ch2)

	outCh := make(chan *Packet, 10)
	ctx := context.Background()

	getPTS := func(pkt *Packet) int64 {
		return pkt.PTS
	}

	go func() {
		_ = barrier.Run(ctx, outCh, getPTS)
	}()

	var output []*Packet
	for pkt := range outCh {
		output = append(output, pkt)
	}

	if len(output) != 6 {
		t.Fatalf("Expected 6 packets, got %d", len(output))
	}

	for i := 0; i < 6; i++ {
		expectedPTS := int64((i + 1) * 10)
		if output[i].PTS != expectedPTS {
			t.Errorf("Packet %d: Expected PTS %d, got %d", i, expectedPTS, output[i].PTS)
		}
	}
}

func TestHTTPRangeReader(t *testing.T) {
	// Create mock content
	data := make([]byte, 100)
	for i := range data {
		data[i] = byte(i)
	}

	// Start local mock HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))

		// Range request handling
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
			return
		}

		if !r.ProtoAtLeast(1, 1) {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Simple parsing of Range bytes=START-
		var start int
		_, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if start >= len(data) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}

		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(data)-1, len(data)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[start:])
	}))
	defer ts.Close()

	reader, err := NewHTTPRangeReader(ts.URL, ts.Client())
	if err != nil {
		t.Fatalf("Failed to initialize remote reader: %v", err)
	}

	if reader.Size() != 100 {
		t.Errorf("Expected reader size to be 100, got %d", reader.Size())
	}

	// Read first 5 bytes
	buf := make([]byte, 5)
	n, err := reader.Read(buf)
	if err != nil || n != 5 {
		t.Fatalf("Expected to read 5 bytes, got n=%d, err=%v", n, err)
	}
	for i := 0; i < 5; i++ {
		if buf[i] != byte(i) {
			t.Errorf("Expected byte %d, got %d", i, buf[i])
		}
	}

	// Seek to offset 50
	offset, err := reader.Seek(50, io.SeekStart)
	if err != nil || offset != 50 {
		t.Fatalf("Expected seek to 50, got offset=%d, err=%v", offset, err)
	}

	// Read next 5 bytes (should trigger new range GET)
	n, err = reader.Read(buf)
	if err != nil || n != 5 {
		t.Fatalf("Expected to read 5 bytes at offset 50, got n=%d, err=%v", n, err)
	}
	for i := 0; i < 5; i++ {
		if buf[i] != byte(50+i) {
			t.Errorf("Expected byte %d, got %d", 50+i, buf[i])
		}
	}

	_ = reader.Close()
}

func TestCGOAndAffinity(t *testing.T) {
	// Simple lookup checks for CGO handle registration
	h := GlobalCGO.Register("test_object")
	if h == 0 {
		t.Error("Expected non-zero handle ID")
	}

	obj, err := GlobalCGO.Lookup(h)
	if err != nil || obj.(string) != "test_object" {
		t.Errorf("Expected 'test_object', got %v (err=%v)", obj, err)
	}

	GlobalCGO.Unregister(h)
	_, err = GlobalCGO.Lookup(h)
	if err == nil {
		t.Error("Expected error after unregistering handle")
	}

	// Call Thread Affinity (should run without panic / build check)
	err = SetThreadAffinity([]int{0})
	// Since we are running in tests and may not have root/syscall permissions
	// depending on environment, we just check that it doesn't crash.
	t.Logf("SetThreadAffinity finished with: %v", err)
}
