package core

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/hajimehoshi/go-mp3"
)

type SimMP3Decoder struct {
	buf bytes.Buffer
	dec *mp3.Decoder
}

func (d *SimMP3Decoder) Decode(pkt *Packet) (*AudioFrame, error) {
	if pkt == nil || len(pkt.Data) == 0 {
		return nil, nil
	}

	d.buf.Write(pkt.Data)

	if d.dec == nil {
		var err error
		d.dec, err = mp3.NewDecoder(&d.buf)
		if err != nil {
			// If not enough data for headers, wait for more
			return nil, nil
		}
	}

	var floats []float32
	tmp := make([]byte, 4096)
	for {
		n, err := d.dec.Read(tmp)
		if n > 0 {
			numSamples := n / 2
			for i := 0; i < numSamples; i++ {
				val := int16(tmp[i*2]) | (int16(tmp[i*2+1]) << 8)
				floats = append(floats, float32(val)/32768.0)
			}
		}
		if err != nil {
			break
		}
	}

	if len(floats) == 0 {
		return nil, nil
	}

	return &AudioFrame{
		Channels:   2,
		SampleRate: d.dec.SampleRate(),
		Data:       floats,
	}, nil
}

func (d *SimMP3Decoder) Close() error {
	return nil
}

// DecodeMP3File decodes an entire MP3 file into a single AudioFrame.
func DecodeMP3File(path string) (*AudioFrame, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open MP3 file: %w", err)
	}
	defer f.Close()

	dec, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, fmt.Errorf("failed to create MP3 decoder: %w", err)
	}

	var floats []float32
	tmp := make([]byte, 8192)
	for {
		n, err := dec.Read(tmp)
		if n > 0 {
			numSamples := n / 2
			for i := 0; i < numSamples; i++ {
				val := int16(tmp[i*2]) | (int16(tmp[i*2+1]) << 8)
				floats = append(floats, float32(val)/32768.0)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("error reading MP3 frames: %w", err)
		}
	}

	return &AudioFrame{
		Channels:   2,
		SampleRate: dec.SampleRate(),
		Data:       floats,
	}, nil
}
