package plugins

import (
	"errors"
	"sync"
	"testing"
)

type mockHandle struct {
	symbols map[string]interface{}
	closed  bool
}

func (m *mockHandle) LookupSymbol(name string) (interface{}, error) {
	sym, ok := m.symbols[name]
	if !ok {
		return nil, errors.New("symbol not found")
	}
	return sym, nil
}

func (m *mockHandle) Close() error {
	m.closed = true
	return nil
}

func TestIsValidSemVer(t *testing.T) {
	valid := []string{"1.0.0", "0.1.0", "12.34.567"}
	invalid := []string{"1", "1.0", "1.0.0.0", "a.b.c", "1.0.a", "1..0", ""}

	for _, v := range valid {
		if !IsValidSemVer(v) {
			t.Errorf("expected %q to be valid SemVer", v)
		}
	}
	for _, v := range invalid {
		if IsValidSemVer(v) {
			t.Errorf("expected %q to be invalid SemVer", v)
		}
	}
}

func TestVerifyPluginABI(t *testing.T) {
	// 1. Valid metadata as struct pointer
	h1 := &mockHandle{
		symbols: map[string]interface{}{
			"PluginMetadata": &PluginMetadata{
				Name:       "test-plugin",
				Version:    "1.2.3",
				ABIVersion: CurrentABIVersion,
			},
		},
	}
	if err := verifyPluginABI(h1); err != nil {
		t.Errorf("expected verifyPluginABI to succeed, got %v", err)
	}

	// 2. Incompatible ABI
	h2 := &mockHandle{
		symbols: map[string]interface{}{
			"PluginMetadata": &PluginMetadata{
				Name:       "test-plugin",
				Version:    "1.2.3",
				ABIVersion: "wrong-abi",
			},
		},
	}
	if err := verifyPluginABI(h2); !errors.Is(err, ErrPluginNotCompatible) {
		t.Errorf("expected ErrPluginNotCompatible, got %v", err)
	}

	// 3. Invalid SemVer version
	h3 := &mockHandle{
		symbols: map[string]interface{}{
			"PluginMetadata": &PluginMetadata{
				Name:       "test-plugin",
				Version:    "invalid-semver",
				ABIVersion: CurrentABIVersion,
			},
		},
	}
	if err := verifyPluginABI(h3); !errors.Is(err, ErrPluginNotCompatible) {
		t.Errorf("expected ErrPluginNotCompatible due to invalid SemVer, got %v", err)
	}

	// 4. Using function GetPluginMetadata
	h4 := &mockHandle{
		symbols: map[string]interface{}{
			"GetPluginMetadata": func() PluginMetadata {
				return PluginMetadata{
					Name:       "test-plugin-func",
					Version:    "2.0.1",
					ABIVersion: CurrentABIVersion,
				}
			},
		},
	}
	if err := verifyPluginABI(h4); err != nil {
		t.Errorf("expected function-based verification to succeed, got %v", err)
	}

	// 5. Missing metadata (treated as allowed for compatibility)
	h5 := &mockHandle{
		symbols: map[string]interface{}{},
	}
	if err := verifyPluginABI(h5); err != nil {
		t.Errorf("expected missing metadata verification to succeed, got %v", err)
	}
}

func TestPluginUnload(t *testing.T) {
	defer UnloadAllPlugins()

	path := "dummy_plugin_path.so"
	handle := &mockHandle{
		symbols: map[string]interface{}{},
	}

	pluginsMu.Lock()
	loadedLibraries[path] = handle
	pluginsMu.Unlock()

	// Register some plugins simulating they were registered by this library
	pluginsMu.Lock()
	currentLoadingPath = path
	pluginsMu.Unlock()

	dp := &MockDemuxerPlugin{}
	RegisterDemuxer(dp)

	dec := &MockDecoderPlugin{}
	RegisterDecoder(dec)

	pluginsMu.Lock()
	currentLoadingPath = ""
	pluginsMu.Unlock()

	// Check they were registered
	if _, ok := GetDemuxer("mock_demux"); !ok {
		t.Fatal("expected demuxer plugin to be registered")
	}

	// Unload the plugin
	if err := UnloadPlugin(path); err != nil {
		t.Fatalf("failed to unload plugin: %v", err)
	}

	if !handle.closed {
		t.Error("expected plugin handle to be closed on unload")
	}

	// Check they were removed from registry
	if _, ok := GetDemuxer("mock_demux"); ok {
		t.Error("expected demuxer plugin to be unregistered after unload")
	}
	pluginsMu.RLock()
	_, decExists := decoderPlugins["mock_dec"]
	pluginsMu.RUnlock()
	if decExists {
		t.Error("expected decoder plugin to be unregistered after unload")
	}
}

func TestConcurrentPluginLoad(t *testing.T) {
	// Verify parallel loading of a plugin does not cause race/deadlock
	var wg sync.WaitGroup
	const workers = 10
	errChan := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Should return error since the file doesn't exist, but should not deadlock
			err := LoadPluginDynamic("concurrent_non_existent.so")
			if err == nil {
				errChan <- errors.New("expected error loading non-existent plugin")
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Error(err)
	}
}
