# 📊 Relatório Comparativo de Benchmark: CroMedia vs FFmpeg (v2)

**Data**: 2026-06-23 18:22:43
**Total de Testes**: 100

---

## 📈 Resumo Executivo

| Métrica | CroMedia | FFmpeg | Diferença |
|---------|----------|--------|-----------|
| **Tempo Total** | 6853 ms | 17901 ms | **2.6x mais rápido** |
| **Memória Total** | 1071.7 MB | 7268.2 MB | **6.8x menos memória** |
| **Memória Média/Teste** | 10.7 MB | 72.7 MB | **6.8x** |

## 📊 Desempenho por Categoria

| Categoria | Speedup Médio | Ratio Memória | Testes |
|-----------|:-------------:|:-------------:|:------:|
| Metadata Probing | **11.3x** | **20.1x** | 10 |
| Tags & Chapters | **8.1x** | **22.7x** | 10 |
| Remuxing & Cutting | **5.4x** | **7.2x** | 15 |
| Video Processing | **2.7x** | **5.9x** | 20 |
| Audio Processing | **2.8x** | **6.9x** | 20 |
| Hardware Acceleration | **2.7x** | **5.9x** | 15 |
| Networking & Telemetry | **2.6x** | **7.3x** | 10 |

### Metadata Probing

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 001 | Probe MP4 Container Structure | 1 ms / 4.2 MB | 20 ms / 86.3 MB | **20.0x** | 20.4x | SUCCESS |
| 002 | Probe WebM Container Structure | 2 ms / 2.7 MB | 22 ms / 75.1 MB | **11.0x** | 27.5x | SUCCESS |
| 003 | Probe MPEG-TS Container Structure | 2 ms / 3.5 MB | 32 ms / 79.5 MB | **16.0x** | 23.1x | SUCCESS |
| 004 | Probe FLV Container Structure | 1 ms / 3.2 MB | 13 ms / 42.7 MB | **13.0x** | 13.3x | SUCCESS |
| 005 | Probe Ogg Container Structure | 1 ms / 3.3 MB | 13 ms / 96.3 MB | **13.0x** | 29.1x | SUCCESS |
| 006 | Probe WAV Container Structure | 1 ms / 3.8 MB | 5 ms / 83.5 MB | **5.0x** | 22.0x | SUCCESS |
| 007 | Probe MP3 Container Structure | 1 ms / 3.0 MB | 9 ms / 58.3 MB | **9.0x** | 19.2x | SUCCESS |
| 008 | Probe FLAC Container Structure | 1 ms / 3.8 MB | 12 ms / 48.0 MB | **12.0x** | 12.7x | SUCCESS |
| 009 | Probe Annex B Container Structure | 2 ms / 3.1 MB | 24 ms / 52.3 MB | **12.0x** | 16.8x | SUCCESS |
| 010 | Fast Sniff Format Identification | 1 ms / 4.3 MB | 2 ms / 72.9 MB | **2.0x** | 16.8x | SUCCESS |

### Tags & Chapters

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 011 | Extract MP4 Metadata Tags (ilst) | 1 ms / 2.5 MB | 9 ms / 57.3 MB | **9.0x** | 22.6x | SUCCESS |
| 012 | Extract Matroska Metadata Tags | 1 ms / 3.6 MB | 10 ms / 70.3 MB | **10.0x** | 19.5x | SUCCESS |
| 013 | Extract MP4 Chapters (chpl) | 1 ms / 4.0 MB | 8 ms / 54.6 MB | **8.0x** | 13.6x | SUCCESS |
| 014 | Extract Matroska Chapters (EBML) | 1 ms / 3.8 MB | 9 ms / 62.3 MB | **9.0x** | 16.3x | SUCCESS |
| 015 | Read HDR10 Metadata (colr/clli) | 1 ms / 4.4 MB | 11 ms / 82.5 MB | **11.0x** | 18.8x | SUCCESS |
| 016 | Extract Codec Private SPS/PPS | 1 ms / 2.2 MB | 5 ms / 82.5 MB | **5.0x** | 36.9x | SUCCESS |
| 017 | Read Rotation Matrix (tkhd) | 1 ms / 3.9 MB | 3 ms / 71.4 MB | **3.0x** | 18.4x | SUCCESS |
| 018 | Read WebP Animated Loop Count | 1 ms / 4.4 MB | 5 ms / 45.8 MB | **5.0x** | 10.5x | SUCCESS |
| 019 | AV1 OBU Sequence Header Parse | 1 ms / 2.4 MB | 14 ms / 68.9 MB | **14.0x** | 29.3x | SUCCESS |
| 020 | ProRes Frame Header ID | 1 ms / 2.1 MB | 7 ms / 85.1 MB | **7.0x** | 41.3x | SUCCESS |

### Remuxing & Cutting

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 021 | CFR Video Remux Cut (0s to 10s) | 42 ms / 8.9 MB | 109 ms / 92.5 MB | **2.6x** | 10.3x | SUCCESS |
| 022 | VFR Video Remux Cut (10s to 20s) | 50 ms / 6.2 MB | 122 ms / 83.2 MB | **2.4x** | 13.4x | SUCCESS |
| 023 | O(log N) Seeking Snap-to-Keyframe | 1 ms / 11.9 MB | 45 ms / 47.1 MB | **45.0x** | 4.0x | SUCCESS |
| 024 | Multi-track Audio/Video Remux | 66 ms / 13.0 MB | 148 ms / 85.6 MB | **2.2x** | 6.6x | SUCCESS |
| 025 | Multi-track Subtitle/Video Remux | 57 ms / 8.9 MB | 172 ms / 89.5 MB | **3.0x** | 10.0x | SUCCESS |
| 026 | Co64 64-bit Offset Remuxing | 58 ms / 10.1 MB | 180 ms / 52.8 MB | **3.1x** | 5.2x | SUCCESS |
| 027 | Ctts Table Interleaved Writing | 79 ms / 8.0 MB | 163 ms / 96.2 MB | **2.1x** | 12.0x | SUCCESS |
| 028 | Edts/Elst Lip-Sync Correction | 78 ms / 12.6 MB | 211 ms / 78.9 MB | **2.7x** | 6.3x | SUCCESS |
| 029 | fMP4 Segment Generation | 49 ms / 7.5 MB | 116 ms / 57.6 MB | **2.4x** | 7.7x | SUCCESS |
| 030 | Annex B Stream Extraction | 53 ms / 14.4 MB | 95 ms / 59.9 MB | **1.8x** | 4.2x | SUCCESS |
| 031 | SRT Subtitle Muxing | 6 ms / 13.2 MB | 20 ms / 65.8 MB | **3.3x** | 5.0x | SUCCESS |
| 032 | WebVTT Subtitle Muxing | 8 ms / 18.1 MB | 21 ms / 56.9 MB | **2.6x** | 3.1x | SUCCESS |
| 033 | MP4 FastStart (moov relocation) | 87 ms / 12.2 MB | 228 ms / 62.3 MB | **2.6x** | 5.1x | SUCCESS |
| 034 | MKV Global Tags Serialization | 22 ms / 8.2 MB | 57 ms / 73.0 MB | **2.6x** | 8.8x | SUCCESS |
| 035 | Packet Deduplication Check | 5 ms / 17.0 MB | 16 ms / 96.6 MB | **3.2x** | 5.7x | SUCCESS |

### Video Processing

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 036 | Scale Video Nearest Neighbor (1080p) | 205 ms / 10.0 MB | 562 ms / 93.1 MB | **2.7x** | 9.3x | SUCCESS |
| 037 | Scale Video Bilinear (1080p to 720p) | 406 ms / 8.7 MB | 785 ms / 43.5 MB | **1.9x** | 5.0x | SUCCESS |
| 038 | Crop Video Frame Rect (1080p) | 176 ms / 9.9 MB | 405 ms / 97.8 MB | **2.3x** | 9.9x | SUCCESS |
| 039 | Flip Horizontal Frame (1080p) | 122 ms / 13.8 MB | 409 ms / 66.2 MB | **3.4x** | 4.8x | SUCCESS |
| 040 | Flip Vertical Frame (1080p) | 156 ms / 10.6 MB | 351 ms / 78.1 MB | **2.2x** | 7.4x | SUCCESS |
| 041 | Rotate Frame 90 Degrees | 215 ms / 19.8 MB | 554 ms / 66.4 MB | **2.6x** | 3.4x | SUCCESS |
| 042 | Rotate Frame 180 Degrees | 209 ms / 11.5 MB | 507 ms / 80.4 MB | **2.4x** | 7.0x | SUCCESS |
| 043 | Rotate Frame 270 Degrees | 203 ms / 9.7 MB | 523 ms / 73.4 MB | **2.6x** | 7.5x | SUCCESS |
| 044 | Adjust Brightness & Contrast | 142 ms / 13.9 MB | 436 ms / 42.9 MB | **3.1x** | 3.1x | SUCCESS |
| 045 | SIMD Sobel Edge Detection | 288 ms / 15.3 MB | 1244 ms / 95.2 MB | **4.3x** | 6.2x | SUCCESS |
| 046 | Generate Color Bars Test Pattern | 25 ms / 12.2 MB | 66 ms / 66.9 MB | **2.6x** | 5.5x | SUCCESS |
| 047 | Bicubic Scale (1080p to 720p) | 369 ms / 15.0 MB | 869 ms / 61.6 MB | **2.4x** | 4.1x | SUCCESS |
| 048 | SIMD/AVX2 Batch Scale Optimization | 59 ms / 12.4 MB | 265 ms / 47.9 MB | **4.5x** | 3.9x | SUCCESS |
| 049 | Vignette Mask Application | 200 ms / 19.5 MB | 509 ms / 86.3 MB | **2.5x** | 4.4x | SUCCESS |
| 050 | Deinterlacing Bob Mode Filter | 225 ms / 18.8 MB | 402 ms / 75.0 MB | **1.8x** | 4.0x | SUCCESS |
| 051 | Deinterlacing Weave Mode Filter | 225 ms / 13.3 MB | 424 ms / 92.7 MB | **1.9x** | 7.0x | SUCCESS |
| 052 | Color Space Rec601 to Rec709 | 148 ms / 7.0 MB | 386 ms / 53.8 MB | **2.6x** | 7.7x | SUCCESS |
| 053 | Frame Rate CFR Conversion (24->30) | 244 ms / 12.0 MB | 619 ms / 43.7 MB | **2.5x** | 3.6x | SUCCESS |
| 054 | Frame Rate Linear Interpolation | 259 ms / 12.9 MB | 648 ms / 96.8 MB | **2.5x** | 7.5x | SUCCESS |
| 055 | 3D LUT Color Grading Application | 256 ms / 13.2 MB | 699 ms / 88.2 MB | **2.7x** | 6.7x | SUCCESS |

### Audio Processing

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 056 | PCM Audio Decoding (16-bit) | 20 ms / 10.6 MB | 45 ms / 61.2 MB | **2.2x** | 5.8x | SUCCESS |
| 057 | PCM Audio Encoding (16-bit) | 17 ms / 9.0 MB | 47 ms / 42.4 MB | **2.8x** | 4.7x | SUCCESS |
| 058 | Audio Volume Gain Filter | 12 ms / 9.0 MB | 23 ms / 44.8 MB | **1.9x** | 5.0x | SUCCESS |
| 059 | Audio Mute Silence Filter | 4 ms / 8.9 MB | 10 ms / 92.2 MB | **2.5x** | 10.3x | SUCCESS |
| 060 | Sinc Resampling (44.1kHz→48kHz) | 23 ms / 14.4 MB | 91 ms / 51.1 MB | **4.0x** | 3.5x | SUCCESS |
| 061 | Audio Simple Lowpass Filter | 19 ms / 8.8 MB | 66 ms / 83.3 MB | **3.5x** | 9.5x | SUCCESS |
| 062 | Audio Fade-In Effect | 8 ms / 11.2 MB | 17 ms / 86.6 MB | **2.1x** | 7.8x | SUCCESS |
| 063 | Audio Fade-Out Effect | 7 ms / 13.0 MB | 19 ms / 88.5 MB | **2.7x** | 6.8x | SUCCESS |
| 064 | Predictive Gain Normalizer | 10 ms / 6.5 MB | 34 ms / 89.4 MB | **3.4x** | 13.9x | SUCCESS |
| 065 | Generate Sine Wave Audio | 4 ms / 12.2 MB | 11 ms / 66.8 MB | **2.8x** | 5.5x | SUCCESS |
| 066 | Generate White Noise Audio | 3 ms / 6.3 MB | 12 ms / 94.9 MB | **4.0x** | 14.9x | SUCCESS |
| 067 | Sinc Resampler LUT (high quality) | 36 ms / 12.9 MB | 137 ms / 84.2 MB | **3.8x** | 6.5x | SUCCESS |
| 068 | Audio Stereo to Mono Mixing | 11 ms / 10.4 MB | 33 ms / 93.4 MB | **3.0x** | 9.0x | SUCCESS |
| 069 | Audio Mono to Stereo Duplicate | 10 ms / 16.0 MB | 21 ms / 48.6 MB | **2.1x** | 3.0x | SUCCESS |
| 070 | Audio 5.1 to Stereo Downmix | 40 ms / 16.2 MB | 95 ms / 99.0 MB | **2.4x** | 6.1x | SUCCESS |
| 071 | Audio Highpass Cutoff 15kHz | 24 ms / 16.9 MB | 48 ms / 60.2 MB | **2.0x** | 3.6x | SUCCESS |
| 072 | Dynamic Range Compressor | 26 ms / 16.4 MB | 74 ms / 83.4 MB | **2.9x** | 5.1x | SUCCESS |
| 073 | Pink Noise Generator (Voss-McCartney) | 6 ms / 18.3 MB | 16 ms / 75.8 MB | **2.7x** | 4.1x | SUCCESS |
| 074 | AAC ADTS Header Parsing | 4 ms / 19.5 MB | 9 ms / 65.1 MB | **2.2x** | 3.3x | SUCCESS |
| 075 | FLAC Metadata Block Vorbis Comment | 6 ms / 6.3 MB | 20 ms / 61.6 MB | **3.3x** | 9.8x | SUCCESS |

### Hardware Acceleration

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 076 | Hardware Device Detection (CUDA/VAAPI) | 49 ms / 14.3 MB | 151 ms / 59.3 MB | **3.1x** | 4.1x | SUCCESS |
| 077 | VRAM Usage Query Metrics | 15 ms / 9.9 MB | 45 ms / 88.6 MB | **3.0x** | 8.9x | SUCCESS |
| 078 | NVENC Session Initialization | 75 ms / 9.2 MB | 217 ms / 84.7 MB | **2.9x** | 9.2x | SUCCESS |
| 079 | RAM to VRAM CUDA Upload | 43 ms / 19.1 MB | 124 ms / 73.3 MB | **2.9x** | 3.8x | SUCCESS |
| 080 | NVDEC GPU Decoding Support | 89 ms / 19.2 MB | 229 ms / 67.1 MB | **2.6x** | 3.5x | SUCCESS |
| 081 | Intel QSV Decoder Stub Check | 32 ms / 15.9 MB | 68 ms / 95.9 MB | **2.1x** | 6.0x | SUCCESS |
| 082 | Apple VideoToolbox Accel Check | 23 ms / 19.4 MB | 67 ms / 96.7 MB | **2.9x** | 5.0x | SUCCESS |
| 083 | VAAPI Hardware Muxer Check | 57 ms / 12.8 MB | 159 ms / 75.5 MB | **2.8x** | 5.9x | SUCCESS |
| 084 | DXVA/D3D11 Windows Muxer Check | 59 ms / 13.8 MB | 163 ms / 91.2 MB | **2.8x** | 6.6x | SUCCESS |
| 085 | GPU→CPU Fallback System | 79 ms / 10.2 MB | 222 ms / 62.0 MB | **2.8x** | 6.1x | SUCCESS |
| 086 | GPU Direct Video Transcode | 152 ms / 19.8 MB | 372 ms / 77.4 MB | **2.5x** | 3.9x | SUCCESS |
| 087 | GPU NVENC Preset P1-P7 | 115 ms / 8.3 MB | 317 ms / 99.1 MB | **2.8x** | 11.9x | SUCCESS |
| 088 | GPU Parallel NVDEC Decode | 145 ms / 19.8 MB | 340 ms / 65.0 MB | **2.3x** | 3.3x | SUCCESS |
| 089 | CUDA Video Scaling Filter | 91 ms / 14.2 MB | 227 ms / 97.3 MB | **2.5x** | 6.8x | SUCCESS |
| 090 | CUDA Frame Overlay Blending | 118 ms / 16.3 MB | 232 ms / 58.7 MB | **2.0x** | 3.6x | SUCCESS |

### Networking & Telemetry

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 091 | RTMP Network Streaming Output | 96 ms / 9.1 MB | 238 ms / 55.5 MB | **2.5x** | 6.1x | SUCCESS |
| 092 | RTSP Network Streaming Output | 86 ms / 12.0 MB | 250 ms / 70.8 MB | **2.9x** | 5.9x | SUCCESS |
| 093 | HLS Segmenter (playlist+chunks) | 119 ms / 15.8 MB | 314 ms / 95.4 MB | **2.6x** | 6.0x | SUCCESS |
| 094 | Hybrid Jitter Buffer (RAM+Disk) | 46 ms / 6.1 MB | 95 ms / 84.8 MB | **2.1x** | 13.9x | SUCCESS |
| 095 | Exponential Backoff Retry Output | 31 ms / 6.5 MB | 95 ms / 89.1 MB | **3.1x** | 13.8x | SUCCESS |
| 096 | RTP Packet Payload Parsing | 25 ms / 11.9 MB | 53 ms / 51.9 MB | **2.1x** | 4.3x | SUCCESS |
| 097 | PCR Clock Sync (500ns tolerance) | 15 ms / 7.9 MB | 49 ms / 42.4 MB | **3.3x** | 5.4x | SUCCESS |
| 098 | WebRTC WHIP Session Handshake | 75 ms / 6.3 MB | 175 ms / 61.2 MB | **2.3x** | 9.7x | SUCCESS |
| 099 | WebRTC WHEP Session Handshake | 67 ms / 19.3 MB | 182 ms / 63.8 MB | **2.7x** | 3.3x | SUCCESS |
| 100 | SDP Offer/Answer Negotiation | 45 ms / 13.4 MB | 90 ms / 65.4 MB | **2.0x** | 4.9x | SUCCESS |

## 🏆 Top 10 Maiores Ganhos de Performance

| # | Caso de Teste | Speedup | CroMedia | FFmpeg |
|---|---------------|:-------:|----------|--------|
| 1 | O(log N) Seeking Snap-to-Keyframe | **45.0x** | 1 ms | 45 ms |
| 2 | Probe MP4 Container Structure | **20.0x** | 1 ms | 20 ms |
| 3 | Probe MPEG-TS Container Structure | **16.0x** | 2 ms | 32 ms |
| 4 | AV1 OBU Sequence Header Parse | **14.0x** | 1 ms | 14 ms |
| 5 | Probe FLV Container Structure | **13.0x** | 1 ms | 13 ms |
| 6 | Probe Ogg Container Structure | **13.0x** | 1 ms | 13 ms |
| 7 | Probe Annex B Container Structure | **12.0x** | 2 ms | 24 ms |
| 8 | Probe FLAC Container Structure | **12.0x** | 1 ms | 12 ms |
| 9 | Probe WebM Container Structure | **11.0x** | 2 ms | 22 ms |
| 10 | Read HDR10 Metadata (colr/clli) | **11.0x** | 1 ms | 11 ms |

