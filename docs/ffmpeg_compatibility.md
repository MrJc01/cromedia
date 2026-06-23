# FFmpeg Syntax Compatibility & Fallback Engine 🎛️

CroMedia includes a syntactic translation wrapper that lets it understand classic FFmpeg CLI arguments. If it cannot process a command natively, it gracefully delegates to the local `ffmpeg` process.

---

## 1. Syntax Mappings & Equivalences

The following options are intercepted by the parser:

| FFmpeg Flag | CroMedia Mapping | Description |
|---|---|---|
| `-i <file>` | `Inputs` | Registers source media input file |
| `-y` | `Overwrite` | Overwrites output files without asking |
| `-c:v <codec>` | `VideoCodec` | Specifies video codec (`copy`, `mjpeg`, `png`, etc) |
| `-c:a <codec>` | `AudioCodec` | Specifies audio codec (`copy`, `pcm`, etc) |
| `-b:v` / `-b:a` | `Bitrates` | Target bitrates for video and audio |
| `-ss` / `-to` / `-t` | `Timings` | Custom range cutting bounds |
| `-vf` / `-af` | `Filters` | Custom audio/video filter chains |
| `-filter_complex` | `FilterComplex` | Custom graph processing chains |
| `-metadata` | `Metadata` | General key=value tags |
| `-chapters` | `Chapters` | Chapters references |
| `-hls_time` / `-hls_list_size` | `HLSFlags` | Network streaming segments options |
| `-rtmp_*` | `RTMPFlags` | Streaming media protocols parameters |
| `--strict` | `Strict` | Restricts fallback execution |
| `--benchmark` | `Benchmark` | Records latency reports |

---

## 2. Fallback Subprocess Engine

If a command is not natively supported (e.g. contains advanced h264 encoding or complex filter graphs), CroMedia's fallback engine takes over:

1. **Detection**: Looks for the `ffmpeg` binary on the host system `PATH`.
2. **CPU Resource Allocation**: Prefixes the command execution with `nice -n 10` and sets CPU environment bounds (`OMP_NUM_THREADS=2`, `GOMAXPROCS=2`) to restrict CPU utilization (Task 149).
3. **Signal Forwarding**: Forwards termination signals (like Ctrl+C / `SIGINT`) to the running subprocess, ensuring no orphaned processes remain (Task 145).
4. **Log Interception**: Captures and routes FFmpeg stderr logs to the main CroMedia log file under the prefix `[FFmpeg Log]` (Task 146).
