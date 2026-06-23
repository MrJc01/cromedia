//go:build cgo_media

package core

/*
#cgo pkg-config: fdk-aac
#include <stdint.h>
#include <stdlib.h>
#include <fdk-aac/aacenc_lib.h>

typedef struct {
    HANDLE_AACENCODER handle;
    AACENC_InfoStruct info;
} cgo_aac_t;

static inline cgo_aac_t* cgo_aac_open(int channels, int sample_rate, int bitrate) {
    cgo_aac_t *ctx = (cgo_aac_t*)malloc(sizeof(cgo_aac_t));
    if (!ctx) return NULL;

    if (aacEncOpen(&ctx->handle, 0, channels) != AACENC_OK) {
        free(ctx);
        return NULL;
    }

    aacEncoder_SetParam(ctx->handle, AACENC_AOT, 2); // AAC-LC
    aacEncoder_SetParam(ctx->handle, AACENC_SAMPLERATE, sample_rate);
    
    CHANNELMODE mode;
    switch (channels) {
        case 1: mode = MODE_1; break;
        case 2: mode = MODE_2; break;
        default: mode = MODE_2; break;
    }
    aacEncoder_SetParam(ctx->handle, AACENC_CHANNELMODE, mode);
    aacEncoder_SetParam(ctx->handle, AACENC_BITRATE, bitrate);
    aacEncoder_SetParam(ctx->handle, AACENC_TRANSMUX, 0); // 0 = RAW (required for MP4), 2 = ADTS

    if (aacEncEncode(ctx->handle, NULL, NULL, NULL, NULL) != AACENC_OK) {
        aacEncClose(&ctx->handle);
        free(ctx);
        return NULL;
    }

    aacEncGetControllerInfo(ctx->handle, &ctx->info);
    return ctx;
}

static inline int cgo_aac_encode(cgo_aac_t *ctx, int16_t *pcm_in, int in_samples, uint8_t *aac_out, int max_out_size) {
    AACENC_BufDesc in_buf_desc = {0};
    AACENC_BufDesc out_buf_desc = {0};
    AACENC_InArgs in_args = {0};
    AACENC_OutArgs out_args = {0};

    int in_identifier = IN_AUDIO_DATA;
    int in_size = in_samples * sizeof(int16_t);
    int in_elem_size = sizeof(int16_t);
    void *in_bufs[] = { pcm_in };

    in_buf_desc.numBufs = 1;
    in_buf_desc.bufs = in_bufs;
    in_buf_desc.bufferIdentifiers = &in_identifier;
    in_buf_desc.bufSizes = &in_size;
    in_buf_desc.bufElSizes = &in_elem_size;

    int out_identifier = OUT_RAW_DATA;
    int out_size = max_out_size;
    int out_elem_size = 1;
    void *out_bufs[] = { aac_out };

    out_buf_desc.numBufs = 1;
    out_buf_desc.bufs = out_bufs;
    out_buf_desc.bufferIdentifiers = &out_identifier;
    out_buf_desc.bufSizes = &out_size;
    out_buf_desc.bufElSizes = &out_elem_size;

    in_args.numInSamples = in_samples;

    AACENC_ERROR err = aacEncEncode(ctx->handle, &in_buf_desc, &out_buf_desc, &in_args, &out_args);
    if (err == AACENC_OK) {
        return out_args.numOutBytes;
    }
    return -((int)err);
}

static inline int cgo_aac_flush(cgo_aac_t *ctx, uint8_t *aac_out, int max_out_size) {
    AACENC_BufDesc out_buf_desc = {0};
    AACENC_InArgs in_args = {0};
    AACENC_OutArgs out_args = {0};

    in_args.numInSamples = -1; // request flush

    int out_identifier = OUT_RAW_DATA;
    int out_size = max_out_size;
    int out_elem_size = 1;
    void *out_bufs[] = { aac_out };

    out_buf_desc.numBufs = 1;
    out_buf_desc.bufs = out_bufs;
    out_buf_desc.bufferIdentifiers = &out_identifier;
    out_buf_desc.bufSizes = &out_size;
    out_buf_desc.bufElSizes = &out_elem_size;

    AACENC_ERROR err = aacEncEncode(ctx->handle, NULL, &out_buf_desc, &in_args, &out_args);
    if (err == AACENC_OK) {
        return out_args.numOutBytes;
    }
    return -((int)err);
}

static inline void cgo_aac_close(cgo_aac_t *ctx) {
    if (!ctx) return;
    aacEncClose(&ctx->handle);
    free(ctx);
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

type SimAACDecoder struct{}

func (d *SimAACDecoder) Decode(pkt *Packet) (*AudioFrame, error) {
	return &AudioFrame{Channels: 2, SampleRate: 44100, Data: make([]float32, 1024)}, nil
}

func (d *SimAACDecoder) Close() error {
	return nil
}

type SimAACEncoder struct {
	ctx        *C.cgo_aac_t
	channels   int
	sampleRate int
	bitrate    int
	pcmBuffer  []int16
	outChan    chan *Packet
}

func (enc *SimAACEncoder) lazyInit(frame *AudioFrame) error {
	if enc.ctx != nil {
		return nil
	}

	ch := 2
	sr := 44100
	if frame != nil {
		ch = frame.Channels
		sr = frame.SampleRate
	}
	enc.channels = ch
	enc.sampleRate = sr
	if enc.bitrate <= 0 {
		enc.bitrate = 128000
	}
	enc.outChan = make(chan *Packet, 64)

	enc.ctx = C.cgo_aac_open(C.int(ch), C.int(sr), C.int(enc.bitrate))
	if enc.ctx == nil {
		return errors.New("failed to initialize fdk-aac encoder in lazy init")
	}

	return nil
}

func (enc *SimAACEncoder) Encode(frame *AudioFrame) (*Packet, error) {
	if frame == nil {
		if enc.ctx == nil {
			return nil, nil
		}
		// Flush: first, process remaining buffered PCM samples by padding with zeros
		frameSamples := 1024 * enc.channels
		if len(enc.pcmBuffer) > 0 {
			padding := make([]int16, frameSamples-len(enc.pcmBuffer))
			enc.pcmBuffer = append(enc.pcmBuffer, padding...)
			if err := enc.encodeChunkDirect(enc.pcmBuffer); err != nil {
				return nil, err
			}
			enc.pcmBuffer = nil
		}

		select {
		case pkt := <-enc.outChan:
			return pkt, nil
		default:
		}

		// Flush the encoder internal buffer
		pkt, err := enc.flushDirect()
		if err != nil {
			return nil, err
		}
		return pkt, nil
	}

	if err := enc.lazyInit(frame); err != nil {
		return nil, err
	}

	// Convert float32 input samples to int16
	for _, f := range frame.Data {
		v := int16(f * 32767.0)
		enc.pcmBuffer = append(enc.pcmBuffer, v)
	}

	// Process in block chunks of 1024 samples per channel
	frameSamples := 1024 * enc.channels
	for len(enc.pcmBuffer) >= frameSamples {
		chunk := enc.pcmBuffer[:frameSamples]
		if err := enc.encodeChunkDirect(chunk); err != nil {
			return nil, err
		}
		enc.pcmBuffer = enc.pcmBuffer[frameSamples:]
	}

	select {
	case pkt := <-enc.outChan:
		return pkt, nil
	default:
		return nil, nil // Buffered
	}
}

func (enc *SimAACEncoder) encodeChunkDirect(chunk []int16) error {
	maxOutSize := 8192
	outBuf := make([]byte, maxOutSize)

	size := C.cgo_aac_encode(
		enc.ctx,
		(*C.int16_t)(unsafe.Pointer(&chunk[0])),
		C.int(len(chunk)),
		(*C.uint8_t)(unsafe.Pointer(&outBuf[0])),
		C.int(maxOutSize),
	)

	if size < 0 {
		return errors.New("failed to encode audio chunk in fdk-aac")
	}

	if size > 0 {
		pktData := make([]byte, size)
		copy(pktData, outBuf[:size])
		pkt := &Packet{
			ID:   NewPacketID(),
			Data: pktData,
		}
		enc.outChan <- pkt
	}

	return nil
}

func (enc *SimAACEncoder) flushDirect() (*Packet, error) {
	maxOutSize := 8192
	outBuf := make([]byte, maxOutSize)

	size := C.cgo_aac_flush(
		enc.ctx,
		(*C.uint8_t)(unsafe.Pointer(&outBuf[0])),
		C.int(maxOutSize),
	)

	if size < 0 {
		return nil, errors.New("failed to flush fdk-aac encoder")
	}

	if size > 0 {
		pktData := make([]byte, size)
		copy(pktData, outBuf[:size])
		return &Packet{
			ID:   NewPacketID(),
			Data: pktData,
		}, nil
	}

	return nil, nil
}

func (enc *SimAACEncoder) Close() error {
	if enc.ctx != nil {
		C.cgo_aac_close(enc.ctx)
		enc.ctx = nil
	}
	return nil
}
