//go:build !cgo_media

package core

type SimAACDecoder struct{}

func (d *SimAACDecoder) Decode(pkt *Packet) (*AudioFrame, error) {
	return &AudioFrame{Channels: 2, SampleRate: 44100, Data: make([]float32, 1024)}, nil
}

func (d *SimAACDecoder) Close() error {
	return nil
}

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
