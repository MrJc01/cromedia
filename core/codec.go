package core

import (
	"errors"
	"sync"
)

// Codec represents metadata about a registered codec.
type Codec struct {
	Name        string
	Type        TrackType
	Description string
}

// DecoderFactory is a function that instantiates a decoder for a specific codec.
type DecoderFactory func() (interface{}, error)

// EncoderFactory is a function that instantiates an encoder for a specific codec.
type EncoderFactory func() (interface{}, error)

var (
	codecsMu sync.RWMutex
	decoders = make(map[string]DecoderFactory)
	encoders = make(map[string]EncoderFactory)
	metadata = make(map[string]Codec)
)

// RegisterCodec registers a codec metadata and optionally its encoder/decoder factories.
func RegisterCodec(meta Codec, dec DecoderFactory, enc EncoderFactory) {
	codecsMu.Lock()
	defer codecsMu.Unlock()

	metadata[meta.Name] = meta
	if dec != nil {
		decoders[meta.Name] = dec
	}
	if enc != nil {
		encoders[meta.Name] = enc
	}
}

// GetDecoder retrieves the decoder factory for the specified codec name.
func GetDecoder(name string) (DecoderFactory, error) {
	codecsMu.RLock()
	defer codecsMu.RUnlock()

	dec, exists := decoders[name]
	if !exists {
		return nil, errors.New("decoder not found for codec: " + name)
	}
	return dec, nil
}

// GetEncoder retrieves the encoder factory for the specified codec name.
func GetEncoder(name string) (EncoderFactory, error) {
	codecsMu.RLock()
	defer codecsMu.RUnlock()

	enc, exists := encoders[name]
	if !exists {
		return nil, errors.New("encoder not found for codec: " + name)
	}
	return enc, nil
}

// ListCodecs returns a list of all registered codecs.
func ListCodecs() []Codec {
	codecsMu.RLock()
	defer codecsMu.RUnlock()

	list := make([]Codec, 0, len(metadata))
	for _, meta := range metadata {
		list = append(list, meta)
	}
	return list
}
