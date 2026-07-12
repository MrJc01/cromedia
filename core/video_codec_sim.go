//go:build !cgo_media

package core


type SimH264Encoder struct {
	KeyintMax int
	fps       int
}

func NewSimH264Encoder(fps int, keyintMax int) *SimH264Encoder {
	return &SimH264Encoder{
		fps:       fps,
		KeyintMax: keyintMax,
	}
}

func (e *SimH264Encoder) Encode(frame *VideoFrame) (*Packet, error) {
	if frame == nil {
		return nil, nil
	}
	return &Packet{ID: NewPacketID(), Data: make([]byte, 1024), IsKeyframe: true}, nil
}

func (e *SimH264Encoder) Close() error {
	return nil
}

type SimH264Decoder struct{}

func (d *SimH264Decoder) Init(codecPrivate []byte) error {
	return nil
}

func (d *SimH264Decoder) Decode(pkt *Packet) (*VideoFrame, error) {
	if pkt == nil {
		return nil, nil
	}
	return &VideoFrame{
		Width:  1920,
		Height: 1080,
		Format: PixelFormatYUV420P,
		Data:   make([]byte, 1920*1080*3/2),
	}, nil
}

func (d *SimH264Decoder) Close() error {
	return nil
}
