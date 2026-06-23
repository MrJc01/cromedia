package pipeline

// Transcoder defines the interface for converting or processing GOPs
type Transcoder interface {
	Transcode(gop *GOP) ([]byte, error)
}

// DummyTranscoder is a placeholder that simulates work and passes data through
type DummyTranscoder struct{}

func (dt *DummyTranscoder) Transcode(gop *GOP) ([]byte, error) {
	totalSize := 0
	for _, s := range gop.Samples {
		totalSize += int(s.Size)
	}

	return make([]byte, totalSize), nil
}
