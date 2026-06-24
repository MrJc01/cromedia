//go:build cgo_media

package core

/*
#cgo pkg-config: x264
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <x264.h>

typedef struct {
    x264_t *x264_enc;
    x264_picture_t pic_in;
    x264_picture_t pic_out;
} cgo_x264_t;

static inline cgo_x264_t* cgo_x264_open(int width, int height, int fps, int bitrate_kbps, const char *preset, const char *tune, int keyint_max) {
    cgo_x264_t *ctx = (cgo_x264_t*)malloc(sizeof(cgo_x264_t));
    if (!ctx) return NULL;

    x264_param_t param;
    x264_param_default_preset(&param, preset ? preset : "veryfast", tune ? tune : "zerolatency");

    param.i_width = width;
    param.i_height = height;
    param.i_fps_num = fps;
    param.i_fps_den = 1;
    param.i_keyint_max = keyint_max > 0 ? keyint_max : fps * 2;
    param.i_csp = X264_CSP_I420;

    if (bitrate_kbps > 0) {
        param.rc.i_rc_method = X264_RC_ABR;
        param.rc.i_bitrate = bitrate_kbps;
    }

    if (x264_param_apply_profile(&param, "high") < 0) {
        free(ctx);
        return NULL;
    }

    ctx->x264_enc = x264_encoder_open(&param);
    if (!ctx->x264_enc) {
        free(ctx);
        return NULL;
    }

    x264_picture_alloc(&ctx->pic_in, X264_CSP_I420, width, height);
    return ctx;
}

static inline void rgba_to_yuv420p(const uint8_t *rgba, int w, int h, x264_picture_t *pic) {
    int y_stride = pic->img.i_stride[0];
    int u_stride = pic->img.i_stride[1];
    int v_stride = pic->img.i_stride[2];
    uint8_t *y_plane = pic->img.plane[0];
    uint8_t *u_plane = pic->img.plane[1];
    uint8_t *v_plane = pic->img.plane[2];

    for (int y = 0; y < h; y++) {
        for (int x = 0; x < w; x++) {
            int rgba_idx = (y * w + x) * 4;
            int r = rgba[rgba_idx];
            int g = rgba[rgba_idx + 1];
            int b = rgba[rgba_idx + 2];

            int Y = ((66 * r + 129 * g + 25 * b + 128) >> 8) + 16;
            y_plane[y * y_stride + x] = (uint8_t)Y;

            if (y % 2 == 0 && x % 2 == 0) {
                int U = ((-38 * r - 74 * g + 112 * b + 128) >> 8) + 128;
                int V = ((112 * r - 94 * g - 18 * b + 128) >> 8) + 128;
                u_plane[(y / 2) * u_stride + (x / 2)] = (uint8_t)U;
                v_plane[(y / 2) * v_stride + (x / 2)] = (uint8_t)V;
            }
        }
    }
}

static inline int cgo_x264_encode(cgo_x264_t *ctx, const uint8_t *rgba, int width, int height, uint8_t *output_buf, int max_output_size, int *is_keyframe, int64_t pts) {
    rgba_to_yuv420p(rgba, width, height, &ctx->pic_in);
    ctx->pic_in.i_pts = pts;

    x264_nal_t *nals;
    int nnal;
    int size = x264_encoder_encode(ctx->x264_enc, &nals, &nnal, &ctx->pic_in, &ctx->pic_out);
    if (size < 0) return size;

    int total_bytes = 0;
    *is_keyframe = 0;
    for (int i = 0; i < nnal; i++) {
        if (total_bytes + nals[i].i_payload <= max_output_size) {
            memcpy(output_buf + total_bytes, nals[i].p_payload, nals[i].i_payload);
            total_bytes += nals[i].i_payload;
        }
        if (nals[i].i_type == NAL_SLICE_IDR) {
            *is_keyframe = 1;
        }
    }
    return total_bytes;
}

static inline int cgo_x264_flush(cgo_x264_t *ctx, uint8_t *output_buf, int max_output_size, int *is_keyframe) {
    x264_nal_t *nals;
    int nnal;
    int size = x264_encoder_encode(ctx->x264_enc, &nals, &nnal, NULL, &ctx->pic_out);
    if (size <= 0) return size;

    int total_bytes = 0;
    *is_keyframe = 0;
    for (int i = 0; i < nnal; i++) {
        if (total_bytes + nals[i].i_payload <= max_output_size) {
            memcpy(output_buf + total_bytes, nals[i].p_payload, nals[i].i_payload);
            total_bytes += nals[i].i_payload;
        }
        if (nals[i].i_type == NAL_SLICE_IDR) {
            *is_keyframe = 1;
        }
    }
    return total_bytes;
}

static inline void cgo_x264_close(cgo_x264_t *ctx) {
    if (!ctx) return;
    x264_picture_clean(&ctx->pic_in);
    x264_encoder_close(ctx->x264_enc);
    free(ctx);
}

static inline void copy_yuv420p(const uint8_t *yuv, int w, int h, x264_picture_t *pic) {
    int y_stride = pic->img.i_stride[0];
    int u_stride = pic->img.i_stride[1];
    int v_stride = pic->img.i_stride[2];
    uint8_t *y_plane = pic->img.plane[0];
    uint8_t *u_plane = pic->img.plane[1];
    uint8_t *v_plane = pic->img.plane[2];

    const uint8_t *y_src = yuv;
    const uint8_t *u_src = yuv + (w * h);
    const uint8_t *v_src = u_src + ((w / 2) * (h / 2));

    for (int y = 0; y < h; y++) {
        memcpy(y_plane + (y * y_stride), y_src + (y * w), w);
    }
    for (int y = 0; y < h / 2; y++) {
        memcpy(u_plane + (y * u_stride), u_src + (y * (w / 2)), w / 2);
        memcpy(v_plane + (y * v_stride), v_src + (y * (w / 2)), w / 2);
    }
}

static inline int cgo_x264_encode_yuv(cgo_x264_t *ctx, const uint8_t *yuv, int width, int height, uint8_t *output_buf, int max_output_size, int *is_keyframe, int64_t pts) {
    copy_yuv420p(yuv, width, height, &ctx->pic_in);
    ctx->pic_in.i_pts = pts;

    x264_nal_t *nals;
    int nnal;
    int size = x264_encoder_encode(ctx->x264_enc, &nals, &nnal, &ctx->pic_in, &ctx->pic_out);
    if (size < 0) return size;

    int total_bytes = 0;
    *is_keyframe = 0;
    for (int i = 0; i < nnal; i++) {
        if (total_bytes + nals[i].i_payload <= max_output_size) {
            memcpy(output_buf + total_bytes, nals[i].p_payload, nals[i].i_payload);
            total_bytes += nals[i].i_payload;
        }
        if (nals[i].i_type == NAL_SLICE_IDR) {
            *is_keyframe = 1;
        }
    }
    return total_bytes;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

type SimH264Encoder struct {
	ctx       *C.cgo_x264_t
	width     int
	height    int
	fps       int
	pts       int64
	batchProc *CGOBatchProcessor
	outChan   chan *Packet
	KeyintMax int
}

func NewSimH264Encoder(fps int, keyintMax int) *SimH264Encoder {
	return &SimH264Encoder{
		fps:       fps,
		KeyintMax: keyintMax,
	}
}

func (enc *SimH264Encoder) lazyInit(frame *VideoFrame) error {
	if enc.ctx != nil {
		return nil
	}

	w := 1920
	h := 1080
	if frame != nil {
		w = frame.Width
		h = frame.Height
	}
	enc.width = w
	enc.height = h
	if enc.fps <= 0 {
		enc.fps = 30
	}
	enc.outChan = make(chan *Packet, 64)

	preset := C.CString("veryfast")
	tune := C.CString("zerolatency")
	defer C.free(unsafe.Pointer(preset))
	defer C.free(unsafe.Pointer(tune))

	enc.ctx = C.cgo_x264_open(C.int(w), C.int(h), C.int(enc.fps), 0, preset, tune, C.int(enc.KeyintMax))
	if enc.ctx == nil {
		return errors.New("failed to initialize x264 encoder in lazy init")
	}

	enc.batchProc = NewCGOBatchProcessor(4, func(batch []*VideoFrame) error {
		for _, f := range batch {
			err := enc.encodeFrameDirect(f)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return nil
}

func (enc *SimH264Encoder) Encode(frame *VideoFrame) (*Packet, error) {
	if frame == nil {
		if enc.ctx == nil {
			return nil, nil
		}
		if enc.batchProc != nil {
			if err := enc.batchProc.Flush(); err != nil {
				return nil, err
			}
		}
		select {
		case pkt := <-enc.outChan:
			return pkt, nil
		default:
		}
		pkt, err := enc.flushDirect()
		if err != nil {
			return nil, err
		}
		return pkt, nil
	}

	if err := enc.lazyInit(frame); err != nil {
		return nil, err
	}

	err := enc.batchProc.AddFrame(frame)
	if err != nil {
		return nil, err
	}

	select {
	case pkt := <-enc.outChan:
		return pkt, nil
	default:
		return nil, nil
	}
}

func (enc *SimH264Encoder) encodeFrameDirect(frame *VideoFrame) error {
	if frame.Format != PixelFormatRGBA && frame.Format != PixelFormatYUV420P {
		return errors.New("cgo x264 encoder expects RGBA or YUV420P frames")
	}

	maxOutSize := frame.Width * frame.Height * 4
	outBuf := make([]byte, maxOutSize)

	var isKeyframe C.int
	var size C.int

	if frame.Format == PixelFormatRGBA {
		size = C.cgo_x264_encode(
			enc.ctx,
			(*C.uint8_t)(unsafe.Pointer(&frame.Data[0])),
			C.int(frame.Width),
			C.int(frame.Height),
			(*C.uint8_t)(unsafe.Pointer(&outBuf[0])),
			C.int(maxOutSize),
			&isKeyframe,
			C.int64_t(enc.pts),
		)
	} else {
		size = C.cgo_x264_encode_yuv(
			enc.ctx,
			(*C.uint8_t)(unsafe.Pointer(&frame.Data[0])),
			C.int(frame.Width),
			C.int(frame.Height),
			(*C.uint8_t)(unsafe.Pointer(&outBuf[0])),
			C.int(maxOutSize),
			&isKeyframe,
			C.int64_t(enc.pts),
		)
	}

	enc.pts++

	if size < 0 {
		return errors.New("failed to encode frame in x264")
	}

	if size > 0 {
		pktData := make([]byte, size)
		copy(pktData, outBuf[:size])
		pkt := &Packet{
			ID:         NewPacketID(),
			Data:       pktData,
			PTS:        enc.pts - 1,
			DTS:        enc.pts - 1,
			IsKeyframe: isKeyframe != 0,
		}
		enc.outChan <- pkt
	}

	return nil
}

func (enc *SimH264Encoder) flushDirect() (*Packet, error) {
	maxOutSize := enc.width * enc.height * 4
	outBuf := make([]byte, maxOutSize)

	var isKeyframe C.int
	size := C.cgo_x264_flush(
		enc.ctx,
		(*C.uint8_t)(unsafe.Pointer(&outBuf[0])),
		C.int(maxOutSize),
		&isKeyframe,
	)

	if size < 0 {
		return nil, errors.New("failed to flush x264 encoder")
	}

	if size > 0 {
		pktData := make([]byte, size)
		copy(pktData, outBuf[:size])
		return &Packet{
			ID:         NewPacketID(),
			Data:       pktData,
			PTS:        enc.pts,
			DTS:        enc.pts,
			IsKeyframe: isKeyframe != 0,
		}, nil
	}

	return nil, nil
}

func (enc *SimH264Encoder) Close() error {
	if enc.ctx != nil {
		C.cgo_x264_close(enc.ctx)
		enc.ctx = nil
	}
	return nil
}
