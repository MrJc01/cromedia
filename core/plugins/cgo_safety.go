package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"plugin"
	"strings"
)

var (
	// ErrAllocationFailed is returned when a CGO allocation returns a nil pointer.
	ErrAllocationFailed = errors.New("cgo allocation failed: buffer returned NULL")
)

// SafeLookupSymbol performs a safe lookup of a plugin symbol, catching any panic that might happen.
func SafeLookupSymbol(p *plugin.Plugin, name string) (sym plugin.Symbol, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered while looking up plugin symbol '%s': %v", name, r)
		}
	}()
	return p.Lookup(name)
}

// CheckCGOBuffer verifies that an interface or pointer represents a valid address.
func CheckCGOBuffer(ptr interface{}) error {
	if ptr == nil {
		return ErrAllocationFailed
	}
	return nil
}

// VerifyPluginSignature verifies the SHA256 checksum of a plugin file matches a corresponding .sha256 file.
func VerifyPluginSignature(pluginPath string) error {
	sigPath := pluginPath + ".sha256"
	expectedHexBytes, err := os.ReadFile(sigPath)
	if err != nil {
		return fmt.Errorf("missing signature file %s: %w", sigPath, err)
	}

	expectedHex := string(expectedHexBytes)
	expectedHex = strings.TrimSpace(expectedHex)

	file, err := os.Open(pluginPath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	calculatedHex := hex.EncodeToString(hash.Sum(nil))
	if calculatedHex != expectedHex {
		return fmt.Errorf("cryptographic signature mismatch: expected %s, calculated %s", expectedHex, calculatedHex)
	}

	return nil
}

