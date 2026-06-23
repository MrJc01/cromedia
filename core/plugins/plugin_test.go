package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"cromedia/core"
	"cromedia/core/demux"
)

// Mock Demuxer Plugin
type MockDemuxer struct{}
func (m *MockDemuxer) Probe() ([]core.Track, error)       { return nil, nil }
func (m *MockDemuxer) ReadPacket() (*core.Packet, error) { return nil, nil }
func (m *MockDemuxer) Close() error                     { return nil }

type MockDemuxerPlugin struct{}
func (m *MockDemuxerPlugin) Name() string                     { return "mock_demux" }
func (m *MockDemuxerPlugin) Extensions() []string             { return []string{"mock"} }
func (m *MockDemuxerPlugin) NewDemuxer(file string) (demux.Demuxer, error) {
	return &MockDemuxer{}, nil
}

// Mock Decoder Plugin
type MockDecoderPlugin struct{}
func (m *MockDecoderPlugin) Name() string                 { return "mock_dec" }
func (m *MockDecoderPlugin) Type() core.TrackType         { return core.TrackTypeVideo }
func (m *MockDecoderPlugin) NewDecoder() (interface{}, error) { return nil, nil }

func TestPluginRegistration(t *testing.T) {
	defer UnloadAllPlugins()

	dp := &MockDemuxerPlugin{}
	RegisterDemuxer(dp)

	p, ok := GetDemuxer("mock_demux")
	if !ok {
		t.Fatal("expected demuxer plugin mock_demux to be registered")
	}
	if p.Name() != "mock_demux" {
		t.Errorf("expected name 'mock_demux', got '%s'", p.Name())
	}

	pExt, ok := GetDemuxer("ext:mock")
	if !ok {
		t.Fatal("expected demuxer plugin ext:mock to be registered")
	}
	if pExt.Name() != "mock_demux" {
		t.Errorf("expected name 'mock_demux' for extension, got '%s'", pExt.Name())
	}

	dec := &MockDecoderPlugin{}
	RegisterDecoder(dec)

	pluginsMu.RLock()
	_, decExists := decoderPlugins["mock_dec"]
	pluginsMu.RUnlock()
	if !decExists {
		t.Fatal("expected decoder plugin mock_dec to be registered")
	}
}

func TestLoadPluginNonExistent(t *testing.T) {
	err := LoadPluginDynamic("non_existent_plugin.so")
	if err == nil {
		t.Fatal("expected error when loading non-existent plugin, got nil")
	}
}

func TestLoadPluginsFromDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cromedia_plugins_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy file with incorrect extension
	dummyFile := filepath.Join(tmpDir, "dummy.txt")
	if err := os.WriteFile(dummyFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Scanning directory should not crash
	err = LoadPluginsFromDir(tmpDir)
	if err != nil {
		t.Errorf("expected nil error scanning empty/dummy dir, got %v", err)
	}
}

func TestLogFromPlugin(t *testing.T) {
	// Call bridge functions and make sure they don't panic
	LogFromPlugin(0, "debug message")
	LogFromPlugin(1, "info message")
	LogFromPlugin(2, "warning message")
	LogFromPlugin(3, "error message")
	LogFromPlugin(99, "unknown level message")
}

