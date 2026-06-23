package plugins

import (
	"io"
)

// GenerateCPluginHeader writes the C header specification for compiling CroMedia plugins in C/C++/Rust.
func GenerateCPluginHeader(w io.Writer) error {
	header := `/* 
 * CroMedia Plugin C Header Interface
 * This header defines the binary interface for building plugins for CroMedia in C, C++, or Rust.
 */

#ifndef CROMEDIA_PLUGIN_H
#define CROMEDIA_PLUGIN_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Plugin Metadata definition matching the host Go struct
typedef struct {
    const char* name;
    const char* version;
    const char* abi_version;
} PluginMetadata;

// Packet struct matching core.Packet
typedef struct {
    int64_t id;
    int32_t stream_index;
    const uint8_t* data;
    int32_t data_size;
    int64_t pts;
    int64_t dts;
    int64_t duration;
    int32_t is_keyframe;
} PluginPacket;

// Video Frame format matching core.PixelFormat
typedef const char* PixelFormat;

// Video Frame struct matching core.VideoFrame
typedef struct {
    int32_t width;
    int32_t height;
    PixelFormat format;
    uint8_t* data;
    int32_t data_size;
} PluginVideoFrame;

// Audio Frame struct matching core.AudioFrame
typedef struct {
    int32_t channels;
    int32_t sample_rate;
    float* data;
    int32_t sample_count;
} PluginAudioFrame;

/* Required entrypoint for plugins that self-register or perform setup */
void InitPlugin(void);

/* Optional entrypoint to retrieve plugin metadata dynamically */
PluginMetadata GetPluginMetadata(void);

#ifdef __cplusplus
}
#endif

#endif /* CROMEDIA_PLUGIN_H */
`
	_, err := io.WriteString(w, header)
	return err
}
