package core

// Transcoder defines the interface for converting or processing GOPs
type Transcoder interface {
	Transcode(gop *GOP) ([]byte, error)
}

// DummyTranscoder is a placeholder that simulates work and passes data through
type DummyTranscoder struct{}

func (dt *DummyTranscoder) Transcode(gop *GOP) ([]byte, error) {
	// In a real scenario, this would decode -> filter -> encode
	// For "Smart Cut", this might just return the raw bytes if no re-encoding needed.
	// But usually, Transcoder implies re-encoding.

	// Simulation:
	totalSize := 0
	for _, s := range gop.Samples {
		totalSize += int(s.Size)
	}

	// Just allocate buffer to simulate output
	return make([]byte, totalSize), nil
}
