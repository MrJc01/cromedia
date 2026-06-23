//go:build !cgo_media

package core


type SimAACEncoder struct{}

func (e *SimAACEncoder) Encode(frame *AudioFrame) (*Packet, error) {
	if frame == nil {
		return nil, nil
	}
	return &Packet{ID: NewPacketID(), Data: make([]byte, 256)}, nil
}

func (e *SimAACEncoder) Close() error {
	return nil
}

type SimAACDecoder struct{}

func (d *SimAACDecoder) Init(config []byte) error {
	return nil
}

func (d *SimAACDecoder) Decode(pkt *Packet) (*AudioFrame, error) {
	if pkt == nil {
		return nil, nil
	}
	return &AudioFrame{
		Channels:   2,
		SampleRate: 44100,
		Data:       make([]float32, 1024),
	}, nil
}

func (d *SimAACDecoder) Close() error {
	return nil
}
