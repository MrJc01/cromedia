package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckCGOBuffer(t *testing.T) {
	err := CheckCGOBuffer(nil)
	if err != ErrAllocationFailed {
		t.Errorf("expected ErrAllocationFailed, got %v", err)
	}

	dummyObj := &struct{}{}
	err = CheckCGOBuffer(dummyObj)
	if err != nil {
		t.Errorf("expected nil error for valid object, got %v", err)
	}
}

func TestSafeLookupSymbolPanic(t *testing.T) {
	// Calling lookup on nil plugin should panic or return error, SafeLookupSymbol should recover it
	_, err := SafeLookupSymbol(nil, "AnySymbol")
	if err == nil {
		t.Fatal("expected error from panic recovery, got nil")
	}
}

func TestVerifyPluginSignature(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cromedia_sig_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	pluginPath := filepath.Join(tmpDir, "test_plugin.so")
	content := []byte("fake plugin binary content")
	if err := os.WriteFile(pluginPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Compute hash
	hash := sha256.New()
	hash.Write(content)
	hexSig := hex.EncodeToString(hash.Sum(nil))

	sigPath := pluginPath + ".sha256"
	if err := os.WriteFile(sigPath, []byte(hexSig), 0644); err != nil {
		t.Fatal(err)
	}

	// Test valid verification
	if err := VerifyPluginSignature(pluginPath); err != nil {
		t.Errorf("expected signature to verify, got error: %v", err)
	}

	// Test invalid signature content
	if err := os.WriteFile(sigPath, []byte("incorrectsignature"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPluginSignature(pluginPath); err == nil {
		t.Error("expected verification error for tampered signature, got nil")
	}

	// Test missing signature file
	os.Remove(sigPath)
	if err := VerifyPluginSignature(pluginPath); err == nil {
		t.Error("expected error for missing signature file, got nil")
	}
}

