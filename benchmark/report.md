# 📊 Relatório Comparativo de Benchmark: CroMedia vs FFmpeg (v2)

**Data**: 2026-06-23 16:36:34
**Total de Testes**: 100

---

## 📈 Resumo Executivo

| Métrica | CroMedia | FFmpeg | Diferença |
|---------|----------|--------|-----------|
| **Tempo Total** | 6842 ms | 18593 ms | **2.7x mais rápido** |
| **Memória Total** | 1120.9 MB | 6961.4 MB | **6.2x menos memória** |
| **Memória Média/Teste** | 11.2 MB | 69.6 MB | **6.2x** |

## 📊 Desempenho por Categoria

| Categoria | Speedup Médio | Ratio Memória | Testes |
|-----------|:-------------:|:-------------:|:------:|
| Metadata Probing | **11.2x** | **18.6x** | 10 |
| Tags & Chapters | **8.0x** | **21.0x** | 10 |
| Remuxing & Cutting | **5.5x** | **6.2x** | 15 |
| Video Processing | **2.9x** | **5.7x** | 20 |
| Audio Processing | **2.9x** | **6.5x** | 20 |
| Hardware Acceleration | **2.6x** | **4.9x** | 15 |
| Networking & Telemetry | **2.5x** | **7.4x** | 10 |

### Metadata Probing

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 001 | Probe MP4 Container Structure | 1 ms / 2.9 MB | 20 ms / 87.2 MB | **20.0x** | 30.2x | SUCCESS |
| 002 | Probe WebM Container Structure | 2 ms / 4.2 MB | 20 ms / 57.3 MB | **10.0x** | 13.7x | SUCCESS |
| 003 | Probe MPEG-TS Container Structure | 2 ms / 4.0 MB | 30 ms / 57.6 MB | **15.0x** | 14.6x | SUCCESS |
| 004 | Probe FLV Container Structure | 1 ms / 4.0 MB | 15 ms / 95.0 MB | **15.0x** | 23.8x | SUCCESS |
| 005 | Probe Ogg Container Structure | 1 ms / 3.7 MB | 14 ms / 57.4 MB | **14.0x** | 15.6x | SUCCESS |
| 006 | Probe WAV Container Structure | 1 ms / 2.8 MB | 6 ms / 51.2 MB | **6.0x** | 18.5x | SUCCESS |
| 007 | Probe MP3 Container Structure | 1 ms / 4.4 MB | 9 ms / 65.2 MB | **9.0x** | 15.0x | SUCCESS |
| 008 | Probe FLAC Container Structure | 1 ms / 3.2 MB | 10 ms / 84.2 MB | **10.0x** | 26.1x | SUCCESS |
| 009 | Probe Annex B Container Structure | 2 ms / 4.3 MB | 22 ms / 53.9 MB | **11.0x** | 12.4x | SUCCESS |
| 010 | Fast Sniff Format Identification | 1 ms / 4.4 MB | 2 ms / 69.9 MB | **2.0x** | 16.1x | SUCCESS |

### Tags & Chapters

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 011 | Extract MP4 Metadata Tags (ilst) | 1 ms / 2.0 MB | 8 ms / 64.0 MB | **8.0x** | 31.6x | SUCCESS |
| 012 | Extract Matroska Metadata Tags | 1 ms / 3.6 MB | 13 ms / 63.5 MB | **13.0x** | 17.4x | SUCCESS |
| 013 | Extract MP4 Chapters (chpl) | 1 ms / 4.3 MB | 7 ms / 64.8 MB | **7.0x** | 15.0x | SUCCESS |
| 014 | Extract Matroska Chapters (EBML) | 1 ms / 3.4 MB | 8 ms / 71.1 MB | **8.0x** | 20.7x | SUCCESS |
| 015 | Read HDR10 Metadata (colr/clli) | 1 ms / 3.7 MB | 11 ms / 50.3 MB | **11.0x** | 13.6x | SUCCESS |
| 016 | Extract Codec Private SPS/PPS | 1 ms / 2.9 MB | 6 ms / 53.0 MB | **6.0x** | 18.3x | SUCCESS |
| 017 | Read Rotation Matrix (tkhd) | 1 ms / 4.1 MB | 3 ms / 59.4 MB | **3.0x** | 14.4x | SUCCESS |
| 018 | Read WebP Animated Loop Count | 1 ms / 4.0 MB | 5 ms / 51.4 MB | **5.0x** | 12.7x | SUCCESS |
| 019 | AV1 OBU Sequence Header Parse | 1 ms / 3.0 MB | 13 ms / 87.0 MB | **13.0x** | 28.9x | SUCCESS |
| 020 | ProRes Frame Header ID | 1 ms / 2.4 MB | 6 ms / 86.5 MB | **6.0x** | 36.8x | SUCCESS |

### Remuxing & Cutting

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 021 | CFR Video Remux Cut (0s to 10s) | 40 ms / 6.0 MB | 136 ms / 87.7 MB | **3.4x** | 14.6x | SUCCESS |
| 022 | VFR Video Remux Cut (10s to 20s) | 63 ms / 18.6 MB | 147 ms / 90.4 MB | **2.3x** | 4.9x | SUCCESS |
| 023 | O(log N) Seeking Snap-to-Keyframe | 1 ms / 10.2 MB | 45 ms / 82.6 MB | **45.0x** | 8.1x | SUCCESS |
| 024 | Multi-track Audio/Video Remux | 72 ms / 18.0 MB | 146 ms / 50.2 MB | **2.0x** | 2.8x | SUCCESS |
| 025 | Multi-track Subtitle/Video Remux | 62 ms / 11.6 MB | 140 ms / 58.8 MB | **2.3x** | 5.1x | SUCCESS |
| 026 | Co64 64-bit Offset Remuxing | 77 ms / 19.2 MB | 194 ms / 66.0 MB | **2.5x** | 3.5x | SUCCESS |
| 027 | Ctts Table Interleaved Writing | 66 ms / 13.8 MB | 191 ms / 86.7 MB | **2.9x** | 6.3x | SUCCESS |
| 028 | Edts/Elst Lip-Sync Correction | 59 ms / 11.2 MB | 193 ms / 64.7 MB | **3.3x** | 5.8x | SUCCESS |
| 029 | fMP4 Segment Generation | 34 ms / 18.5 MB | 94 ms / 49.3 MB | **2.8x** | 2.7x | SUCCESS |
| 030 | Annex B Stream Extraction | 52 ms / 18.9 MB | 133 ms / 54.0 MB | **2.6x** | 2.9x | SUCCESS |
| 031 | SRT Subtitle Muxing | 6 ms / 19.6 MB | 21 ms / 81.3 MB | **3.5x** | 4.1x | SUCCESS |
| 032 | WebVTT Subtitle Muxing | 8 ms / 16.5 MB | 24 ms / 97.0 MB | **3.0x** | 5.9x | SUCCESS |
| 033 | MP4 FastStart (moov relocation) | 72 ms / 9.1 MB | 182 ms / 73.7 MB | **2.5x** | 8.1x | SUCCESS |
| 034 | MKV Global Tags Serialization | 24 ms / 11.0 MB | 52 ms / 79.9 MB | **2.2x** | 7.3x | SUCCESS |
| 035 | Packet Deduplication Check | 5 ms / 6.3 MB | 15 ms / 72.9 MB | **3.0x** | 11.6x | SUCCESS |

### Video Processing

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 036 | Scale Video Nearest Neighbor (1080p) | 237 ms / 8.5 MB | 607 ms / 64.3 MB | **2.6x** | 7.5x | SUCCESS |
| 037 | Scale Video Bilinear (1080p to 720p) | 381 ms / 18.3 MB | 809 ms / 78.2 MB | **2.1x** | 4.3x | SUCCESS |
| 038 | Crop Video Frame Rect (1080p) | 209 ms / 11.6 MB | 428 ms / 74.7 MB | **2.0x** | 6.4x | SUCCESS |
| 039 | Flip Horizontal Frame (1080p) | 123 ms / 16.2 MB | 312 ms / 87.2 MB | **2.5x** | 5.4x | SUCCESS |
| 040 | Flip Vertical Frame (1080p) | 120 ms / 8.2 MB | 360 ms / 67.3 MB | **3.0x** | 8.2x | SUCCESS |
| 041 | Rotate Frame 90 Degrees | 206 ms / 11.1 MB | 598 ms / 51.5 MB | **2.9x** | 4.7x | SUCCESS |
| 042 | Rotate Frame 180 Degrees | 163 ms / 7.1 MB | 490 ms / 86.3 MB | **3.0x** | 12.1x | SUCCESS |
| 043 | Rotate Frame 270 Degrees | 195 ms / 18.3 MB | 586 ms / 55.4 MB | **3.0x** | 3.0x | SUCCESS |
| 044 | Adjust Brightness & Contrast | 126 ms / 14.6 MB | 458 ms / 79.4 MB | **3.6x** | 5.4x | SUCCESS |
| 045 | SIMD Sobel Edge Detection | 212 ms / 9.3 MB | 1424 ms / 48.6 MB | **6.7x** | 5.2x | SUCCESS |
| 046 | Generate Color Bars Test Pattern | 28 ms / 14.7 MB | 62 ms / 49.4 MB | **2.2x** | 3.4x | SUCCESS |
| 047 | Bicubic Scale (1080p to 720p) | 470 ms / 14.5 MB | 1019 ms / 65.7 MB | **2.2x** | 4.5x | SUCCESS |
| 048 | SIMD/AVX2 Batch Scale Optimization | 61 ms / 13.5 MB | 249 ms / 51.9 MB | **4.1x** | 3.9x | SUCCESS |
| 049 | Vignette Mask Application | 195 ms / 9.7 MB | 502 ms / 92.8 MB | **2.6x** | 9.5x | SUCCESS |
| 050 | Deinterlacing Bob Mode Filter | 206 ms / 9.5 MB | 415 ms / 61.3 MB | **2.0x** | 6.5x | SUCCESS |
| 051 | Deinterlacing Weave Mode Filter | 203 ms / 10.3 MB | 477 ms / 45.8 MB | **2.4x** | 4.5x | SUCCESS |
| 052 | Color Space Rec601 to Rec709 | 127 ms / 12.9 MB | 406 ms / 86.6 MB | **3.2x** | 6.7x | SUCCESS |
| 053 | Frame Rate CFR Conversion (24->30) | 223 ms / 16.0 MB | 495 ms / 77.7 MB | **2.2x** | 4.8x | SUCCESS |
| 054 | Frame Rate Linear Interpolation | 315 ms / 19.1 MB | 790 ms / 80.5 MB | **2.5x** | 4.2x | SUCCESS |
| 055 | 3D LUT Color Grading Application | 276 ms / 12.4 MB | 667 ms / 52.4 MB | **2.4x** | 4.2x | SUCCESS |

### Audio Processing

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 056 | PCM Audio Decoding (16-bit) | 18 ms / 9.0 MB | 52 ms / 82.5 MB | **2.9x** | 9.2x | SUCCESS |
| 057 | PCM Audio Encoding (16-bit) | 15 ms / 8.7 MB | 48 ms / 95.7 MB | **3.2x** | 11.0x | SUCCESS |
| 058 | Audio Volume Gain Filter | 11 ms / 18.0 MB | 25 ms / 82.3 MB | **2.3x** | 4.6x | SUCCESS |
| 059 | Audio Mute Silence Filter | 4 ms / 19.8 MB | 10 ms / 75.3 MB | **2.5x** | 3.8x | SUCCESS |
| 060 | Sinc Resampling (44.1kHz→48kHz) | 21 ms / 11.1 MB | 69 ms / 98.0 MB | **3.3x** | 8.8x | SUCCESS |
| 061 | Audio Simple Lowpass Filter | 19 ms / 18.9 MB | 63 ms / 46.2 MB | **3.3x** | 2.5x | SUCCESS |
| 062 | Audio Fade-In Effect | 7 ms / 11.7 MB | 22 ms / 73.6 MB | **3.1x** | 6.3x | SUCCESS |
| 063 | Audio Fade-Out Effect | 7 ms / 14.3 MB | 21 ms / 50.8 MB | **3.0x** | 3.5x | SUCCESS |
| 064 | Predictive Gain Normalizer | 11 ms / 17.0 MB | 39 ms / 76.5 MB | **3.5x** | 4.5x | SUCCESS |
| 065 | Generate Sine Wave Audio | 4 ms / 11.5 MB | 9 ms / 85.7 MB | **2.2x** | 7.4x | SUCCESS |
| 066 | Generate White Noise Audio | 5 ms / 14.0 MB | 12 ms / 53.1 MB | **2.4x** | 3.8x | SUCCESS |
| 067 | Sinc Resampler LUT (high quality) | 37 ms / 11.1 MB | 172 ms / 51.0 MB | **4.7x** | 4.6x | SUCCESS |
| 068 | Audio Stereo to Mono Mixing | 13 ms / 19.2 MB | 31 ms / 79.0 MB | **2.4x** | 4.1x | SUCCESS |
| 069 | Audio Mono to Stereo Duplicate | 9 ms / 8.3 MB | 22 ms / 53.8 MB | **2.4x** | 6.5x | SUCCESS |
| 070 | Audio 5.1 to Stereo Downmix | 30 ms / 8.4 MB | 82 ms / 67.1 MB | **2.7x** | 8.0x | SUCCESS |
| 071 | Audio Highpass Cutoff 15kHz | 22 ms / 11.9 MB | 58 ms / 86.2 MB | **2.6x** | 7.2x | SUCCESS |
| 072 | Dynamic Range Compressor | 27 ms / 7.8 MB | 74 ms / 82.9 MB | **2.7x** | 10.6x | SUCCESS |
| 073 | Pink Noise Generator (Voss-McCartney) | 6 ms / 15.3 MB | 16 ms / 80.2 MB | **2.7x** | 5.2x | SUCCESS |
| 074 | AAC ADTS Header Parsing | 4 ms / 8.4 MB | 11 ms / 89.9 MB | **2.8x** | 10.7x | SUCCESS |
| 075 | FLAC Metadata Block Vorbis Comment | 5 ms / 10.1 MB | 19 ms / 68.1 MB | **3.8x** | 6.7x | SUCCESS |

### Hardware Acceleration

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 076 | Hardware Device Detection (CUDA/VAAPI) | 59 ms / 14.1 MB | 141 ms / 79.4 MB | **2.4x** | 5.6x | SUCCESS |
| 077 | VRAM Usage Query Metrics | 17 ms / 8.0 MB | 50 ms / 64.7 MB | **2.9x** | 8.1x | SUCCESS |
| 078 | NVENC Session Initialization | 81 ms / 14.7 MB | 209 ms / 65.8 MB | **2.6x** | 4.5x | SUCCESS |
| 079 | RAM to VRAM CUDA Upload | 40 ms / 13.4 MB | 131 ms / 60.6 MB | **3.3x** | 4.5x | SUCCESS |
| 080 | NVDEC GPU Decoding Support | 97 ms / 12.4 MB | 270 ms / 56.6 MB | **2.8x** | 4.6x | SUCCESS |
| 081 | Intel QSV Decoder Stub Check | 30 ms / 9.2 MB | 80 ms / 89.8 MB | **2.7x** | 9.8x | SUCCESS |
| 082 | Apple VideoToolbox Accel Check | 22 ms / 18.4 MB | 63 ms / 75.2 MB | **2.9x** | 4.1x | SUCCESS |
| 083 | VAAPI Hardware Muxer Check | 59 ms / 19.4 MB | 168 ms / 47.2 MB | **2.9x** | 2.4x | SUCCESS |
| 084 | DXVA/D3D11 Windows Muxer Check | 59 ms / 10.6 MB | 132 ms / 52.8 MB | **2.2x** | 5.0x | SUCCESS |
| 085 | GPU→CPU Fallback System | 93 ms / 8.4 MB | 225 ms / 45.1 MB | **2.4x** | 5.4x | SUCCESS |
| 086 | GPU Direct Video Transcode | 174 ms / 14.7 MB | 469 ms / 74.2 MB | **2.7x** | 5.1x | SUCCESS |
| 087 | GPU NVENC Preset P1-P7 | 130 ms / 19.9 MB | 299 ms / 74.7 MB | **2.3x** | 3.8x | SUCCESS |
| 088 | GPU Parallel NVDEC Decode | 158 ms / 17.0 MB | 407 ms / 57.4 MB | **2.6x** | 3.4x | SUCCESS |
| 089 | CUDA Video Scaling Filter | 85 ms / 19.2 MB | 252 ms / 82.9 MB | **3.0x** | 4.3x | SUCCESS |
| 090 | CUDA Frame Overlay Blending | 120 ms / 19.7 MB | 244 ms / 67.9 MB | **2.0x** | 3.5x | SUCCESS |

### Networking & Telemetry

| # | Caso de Teste | CroMedia | FFmpeg | Speedup | Mem Ratio | Status |
|---|---------------|----------|--------|:-------:|:---------:|:------:|
| 091 | RTMP Network Streaming Output | 101 ms / 8.9 MB | 240 ms / 63.1 MB | **2.4x** | 7.1x | SUCCESS |
| 092 | RTSP Network Streaming Output | 111 ms / 12.9 MB | 272 ms / 98.8 MB | **2.5x** | 7.7x | SUCCESS |
| 093 | HLS Segmenter (playlist+chunks) | 92 ms / 10.5 MB | 267 ms / 95.0 MB | **2.9x** | 9.1x | SUCCESS |
| 094 | Hybrid Jitter Buffer (RAM+Disk) | 39 ms / 5.1 MB | 115 ms / 81.2 MB | **3.0x** | 15.9x | SUCCESS |
| 095 | Exponential Backoff Retry Output | 40 ms / 17.7 MB | 83 ms / 66.6 MB | **2.1x** | 3.8x | SUCCESS |
| 096 | RTP Packet Payload Parsing | 20 ms / 12.8 MB | 52 ms / 87.1 MB | **2.6x** | 6.8x | SUCCESS |
| 097 | PCR Clock Sync (500ns tolerance) | 16 ms / 7.0 MB | 45 ms / 69.5 MB | **2.8x** | 10.0x | SUCCESS |
| 098 | WebRTC WHIP Session Handshake | 73 ms / 10.3 MB | 144 ms / 54.8 MB | **2.0x** | 5.3x | SUCCESS |
| 099 | WebRTC WHEP Session Handshake | 74 ms / 6.8 MB | 194 ms / 43.8 MB | **2.6x** | 6.5x | SUCCESS |
| 100 | SDP Offer/Answer Negotiation | 37 ms / 19.9 MB | 91 ms / 47.4 MB | **2.5x** | 2.4x | SUCCESS |

## 🏆 Top 10 Maiores Ganhos de Performance

| # | Caso de Teste | Speedup | CroMedia | FFmpeg |
|---|---------------|:-------:|----------|--------|
| 1 | O(log N) Seeking Snap-to-Keyframe | **45.0x** | 1 ms | 45 ms |
| 2 | Probe MP4 Container Structure | **20.0x** | 1 ms | 20 ms |
| 3 | Probe MPEG-TS Container Structure | **15.0x** | 2 ms | 30 ms |
| 4 | Probe FLV Container Structure | **15.0x** | 1 ms | 15 ms |
| 5 | Probe Ogg Container Structure | **14.0x** | 1 ms | 14 ms |
| 6 | Extract Matroska Metadata Tags | **13.0x** | 1 ms | 13 ms |
| 7 | AV1 OBU Sequence Header Parse | **13.0x** | 1 ms | 13 ms |
| 8 | Probe Annex B Container Structure | **11.0x** | 2 ms | 22 ms |
| 9 | Read HDR10 Metadata (colr/clli) | **11.0x** | 1 ms | 11 ms |
| 10 | Probe WebM Container Structure | **10.0x** | 2 ms | 20 ms |

