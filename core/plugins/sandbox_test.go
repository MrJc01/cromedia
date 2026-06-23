package plugins

import (
	"os"
	"testing"
	"time"

	"cromedia/core"
)

func init() {
	RegisterDecoder(&mockNormalDecoderPlugin{})
}

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "sandbox-worker" {
		if len(os.Args) < 5 {
			os.Exit(1)
		}
		RunSandboxWorker(os.Args[2], os.Args[3], os.Args[4])
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type mockPanicDecoder struct{}

func (d *mockPanicDecoder) Decode(pkt *core.Packet) (*core.VideoFrame, error) {
	panic("something went wrong inside decoder")
}

func (d *mockPanicDecoder) Close() error { return nil }

type mockStuckDecoder struct{}

func (d *mockStuckDecoder) Decode(pkt *core.Packet) (*core.VideoFrame, error) {
	select {} // block forever
}

func (d *mockStuckDecoder) Close() error { return nil }

type mockNormalDecoder struct{}

func (d *mockNormalDecoder) Decode(pkt *core.Packet) (*core.VideoFrame, error) {
	return &core.VideoFrame{
		Width:  1920,
		Height: 1080,
		Format: "yuv420p",
		Data:   []byte("raw pixel data"),
	}, nil
}

func (d *mockNormalDecoder) Close() error { return nil }

type mockNormalDecoderPlugin struct{}

func (p *mockNormalDecoderPlugin) Name() string         { return "mock_normal_dec" }
func (p *mockNormalDecoderPlugin) Type() core.TrackType { return core.TrackTypeVideo }
func (p *mockNormalDecoderPlugin) NewDecoder() (interface{}, error) {
	return &mockNormalDecoder{}, nil
}

func TestInProcessSandbox(t *testing.T) {
	// Test panic recovery
	pd := &mockPanicDecoder{}
	sandboxPd := NewInProcessVideoDecoder(pd, 100*time.Millisecond)
	_, err := sandboxPd.Decode(&core.Packet{})
	if err == nil {
		t.Fatal("expected error from panic recovery, got nil")
	}
	if err.Error() != "panic in decoder: something went wrong inside decoder" {
		t.Errorf("unexpected error message: %v", err)
	}

	// Test timeout
	sd := &mockStuckDecoder{}
	sandboxSd := NewInProcessVideoDecoder(sd, 50*time.Millisecond)
	_, err = sandboxSd.Decode(&core.Packet{})
	if err == nil {
		t.Fatal("expected error due to timeout, got nil")
	}
	if err.Error() != "decoder timeout: infinite loop or deadlock detected" {
		t.Errorf("unexpected error message: %v", err)
	}

	// Test normal execution
	nd := &mockNormalDecoder{}
	sandboxNd := NewInProcessVideoDecoder(nd, 100*time.Millisecond)
	frame, err := sandboxNd.Decode(&core.Packet{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if frame.Width != 1920 || string(frame.Data) != "raw pixel data" {
		t.Errorf("unexpected frame returned: %+v", frame)
	}
}

func TestSubprocessSandbox(t *testing.T) {
	// Register the normal decoder plugin so the sandbox worker can load it
	RegisterDecoder(&mockNormalDecoderPlugin{})

	// Launch subprocess decoder
	// Since we are running under "go test", os.Executable() points to the test binary,
	// and our TestMain will intercept the "sandbox-worker" arguments.
	sdec, err := NewSubprocessVideoDecoder("", "mock_normal_dec")
	if err != nil {
		t.Fatalf("failed to start subprocess sandbox decoder: %v", err)
	}
	defer sdec.Close()

	pkt := &core.Packet{
		ID:   42,
		Data: []byte("encoded packet bytes"),
	}

	frame, err := sdec.Decode(pkt)
	if err != nil {
		t.Fatalf("failed to decode via subprocess: %v", err)
	}

	if frame.Width != 1920 || frame.Height != 1080 {
		t.Errorf("unexpected frame width/height: %dx%d", frame.Width, frame.Height)
	}
	if string(frame.Data) != "raw pixel data" {
		t.Errorf("unexpected frame data: %s", string(frame.Data))
	}
}
