# libavfilter CGO Bridge Compilation & Usage Guide 🔌

CroMedia provides an optional CGO-based integration bridge to FFmpeg's `libavfilter` C library.

---

## 1. Prerequisites & Dependencies

To compile CroMedia with the CGO filter bridge enabled, you must have the development files for `libavfilter` and `libavutil` installed on your machine.

### Linux (Ubuntu/Debian)
```bash
sudo apt-get update
sudo apt-get install libavfilter-dev libavutil-dev pkg-config
```

### macOS (Homebrew)
```bash
brew install ffmpeg pkg-config
```

---

## 2. Compilation Instructions

By default, the CGO bridge is stubbed out to keep the compiled binary free of C links. To enable CGO bridge compilation, add the build tag `-tags "libavfilter"` when executing Go commands:

### Build binary
```bash
go build -tags "libavfilter" -o cromedia
```

### Run tests and benchmarks
```bash
go test -tags "libavfilter" ./core/filters/bridge/... -v
```

---

## 3. Architecture & Thread-Safety Guarantees
- **Thread Locking**: The bridge automatically locks execution to the OS thread context using `runtime.LockOSThread()`, preventing concurrency mismatches within C runtime context.
- **Zero-Copy Pass**: For planar YUV420p formats, raw pointer addresses are passed directly to C layers to eliminate buffer copy overhead.
