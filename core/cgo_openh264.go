//go:build cgo_media

package core

/*
#cgo pkg-config: openh264
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <wels/codec_api.h>

typedef struct {
    ISVCDecoder *decoder;
} cgo_h264_dec_t;

typedef struct {
    uint8_t *planes[3];
    int stride[3];
    int width;
    int height;
    int ready;
} cgo_h264_frame_t;

static inline cgo_h264_dec_t* cgo_h264_dec_open() {
    cgo_h264_dec_t *ctx = (cgo_h264_dec_t*)malloc(sizeof(cgo_h264_dec_t));
    if (!ctx) return NULL;
    
    if (WelsCreateDecoder(&ctx->decoder) != 0) {
        free(ctx);
        return NULL;
    }
    
    SDecodingParam param = {0};
    param.sVideoProperty.eVideoBsType = VIDEO_BITSTREAM_DEFAULT;
    
    if (ctx->decoder->lpVtbl->Initialize(ctx->decoder, &param) != 0) {
        WelsDestroyDecoder(ctx->decoder);
        free(ctx);
        return NULL;
    }
    
    return ctx;
}

static inline int cgo_h264_dec_decode(cgo_h264_dec_t *ctx, const uint8_t *data, int len, cgo_h264_frame_t *out_frame) {
    out_frame->ready = 0;
    unsigned char *planes[3] = {NULL};
    SBufferInfo info = {0};
    
    DECODING_STATE state = ctx->decoder->lpVtbl->DecodeFrameNoDelay(ctx->decoder, data, len, planes, &info);
    if (state != dsErrorFree) {
        return -((int)state);
    }
    
    if (info.iBufferStatus != 1) {
        return 0; // Frame not ready
    }
    
    out_frame->width = info.UsrData.sSystemBuffer.iWidth;
    out_frame->height = info.UsrData.sSystemBuffer.iHeight;
    out_frame->planes[0] = planes[0];
    out_frame->planes[1] = planes[1];
    out_frame->planes[2] = planes[2];
    out_frame->stride[0] = info.UsrData.sSystemBuffer.iStride[0];
    out_frame->stride[1] = info.UsrData.sSystemBuffer.iStride[1];
    out_frame->stride[2] = info.UsrData.sSystemBuffer.iStride[1];
    out_frame->ready = 1;
    
    return 1;
}

static inline void cgo_h264_dec_copy_frame(cgo_h264_frame_t *frame, uint8_t *dst) {
    int w = frame->width;
    int h = frame->height;
    int y_stride = frame->stride[0];
    int uv_stride = frame->stride[1];
    
    uint8_t *dst_y = dst;
    uint8_t *dst_u = dst + w * h;
    uint8_t *dst_v = dst + w * h + (w * h) / 4;
    
    for (int y = 0; y < h; y++) {
        memcpy(dst_y + y * w, frame->planes[0] + y * y_stride, w);
    }
    for (int y = 0; y < h / 2; y++) {
        memcpy(dst_u + y * (w / 2), frame->planes[1] + y * uv_stride, w / 2);
    }
    for (int y = 0; y < h / 2; y++) {
        memcpy(dst_v + y * (w / 2), frame->planes[2] + y * uv_stride, w / 2);
    }
}

static inline void cgo_h264_dec_close(cgo_h264_dec_t *ctx) {
    if (!ctx) return;
    ctx->decoder->lpVtbl->Uninitialize(ctx->decoder);
    WelsDestroyDecoder(ctx->decoder);
    free(ctx);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

type SimH264Decoder struct {
	ctx *C.cgo_h264_dec_t
}

func (d *SimH264Decoder) Decode(pkt *Packet) (*VideoFrame, error) {
	if pkt == nil || len(pkt.Data) == 0 {
		return nil, nil
	}

	if d.ctx == nil {
		d.ctx = C.cgo_h264_dec_open()
		if d.ctx == nil {
			return nil, errors.New("failed to open openh264 decoder")
		}
	}

	var frame C.cgo_h264_frame_t
	ret := C.cgo_h264_dec_decode(
		d.ctx,
		(*C.uint8_t)(unsafe.Pointer(&pkt.Data[0])),
		C.int(len(pkt.Data)),
		&frame,
	)

	if ret < 0 {
		return nil, fmt.Errorf("openh264 decoding error: %d", ret)
	}

	if frame.ready == 0 {
		return nil, nil // Frame not ready (need more NALs)
	}

	w := int(frame.width)
	h := int(frame.height)
	realSize := w * h * 3 / 2

	// Allocate from the BufferPool
	yuvData := GlobalGet(realSize)
	C.cgo_h264_dec_copy_frame(&frame, (*C.uint8_t)(unsafe.Pointer(&yuvData[0])))

	return &VideoFrame{
		Width:  w,
		Height: h,
		Format: PixelFormatYUV420P,
		Data:   yuvData,
	}, nil
}

func (d *SimH264Decoder) Close() error {
	if d.ctx != nil {
		C.cgo_h264_dec_close(d.ctx)
		d.ctx = nil
	}
	return nil
}
