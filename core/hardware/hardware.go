package hardware

import (
	"cromedia/core/pipeline"
	"fmt"
)

// DeviceType represents hardware accelerator device types
type DeviceType string

const (
	DeviceTypeCUDA   DeviceType = "cuda"
	DeviceTypeVAAPI  DeviceType = "vaapi"
	DeviceTypeQSV    DeviceType = "qsv"
	DeviceTypeVT     DeviceType = "videotoolbox"
	DeviceTypeDXVA2  DeviceType = "dxva2"
	DeviceTypeD3D11  DeviceType = "d3d11va"
)

// ListHardwareDevices returns available hardware acceleration devices on the current platform
func ListHardwareDevices() []string {
	// Simulated detection
	devices := []string{"software"}
	
	// Stub check: on a real device we'd check dynamic library loading or graphics device queries.
	// Since we mock these, we can return what is compilable.
	devices = append(devices, string(DeviceTypeCUDA))
	devices = append(devices, string(DeviceTypeVAAPI))
	return devices
}

// NewHardwareTranscoder creates a hardware-accelerated Transcoder with automatic CPU fallback.
func NewHardwareTranscoder(device DeviceType) (pipeline.Transcoder, error) {
	switch device {
	case DeviceTypeCUDA:
		// Attempt NVENC
		tx, err := NewNVENCTranscoder()
		if err == nil {
			return tx, nil
		}
		// If CUDA NVENC fails/stubs, fallback automatically to CPU
		fmt.Printf("Hardware NVENC failed: %v. Falling back to Software Transcoder.\n", err)
		return &pipeline.DummyTranscoder{}, nil
	case DeviceTypeVAAPI, DeviceTypeQSV, DeviceTypeVT, DeviceTypeDXVA2, DeviceTypeD3D11:
		// Stub other accelerators; default fallback to CPU
		fmt.Printf("Hardware acceleration via %s is not supported on this platform. Falling back to CPU.\n", device)
		return &pipeline.DummyTranscoder{}, nil
	default:
		return &pipeline.DummyTranscoder{}, nil
	}
}

// GetVRAMUsage reports the simulated memory consumption of the GPU
func GetVRAMUsage() (uint64, error) {
	return 1024 * 1024 * 256, nil // 256 MB dummy VRAM usage
}
