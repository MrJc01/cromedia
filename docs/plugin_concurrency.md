# CroMedia Plugin Concurrency & Thread-Safety Rules 🧵

This guide documents the thread-safety guarantees and concurrency constraints that developers of CroMedia plugins must adhere to when writing dynamic decoders, encoders, demuxers, or filters.

---

## 1. Thread Safety in Host-Plugin Boundaries
All plugin instances registered within CroMedia (`DemuxerPlugin`, `DecoderPlugin`, `MuxerPlugin`, `EncoderPlugin`) must be thread-safe for parallel instantiation. The host process may call `NewDemuxer` or `NewDecoder` concurrently from multiple pipelines/goroutines.

## 2. Decoder & Encoder Context Isolation
- **Instance-Level Safety**: An individual decoder or encoder context created by a plugin (`NewDecoder()`) is **not** required to be thread-safe for concurrent calls to `Decode()`. The pipeline engine guarantees that a single codec context will only have its `Decode()` method invoked sequentially from one pipeline execution thread/goroutine.
- **State Management**: Do not share mutable state between multiple decoder instances. If global state is necessary (e.g. library-wide lookup tables), it must be guarded by a thread-safe mutex or initialized at startup.

## 3. Dynamic Memory Allocations & CGO Boundary
- When allocating memory inside C/C++ or Rust libraries loaded via CGO, use `plugins.AllocCGOBuffer` to get contiguous, Go-runtime tracked memory segments or clean up all allocated heap blocks on context termination.
- Failure to call `Free()` on buffers or release handles will trigger memory leak alerts in the build pipelines.

## 4. Subprocess Sandboxing & Thread Limits
- If a plugin is executed under sandboxed mode (isolated process), the thread count is automatically governed by the environment variable `CROMEDIA_PLUGIN_MAX_THREADS` (which sets `GOMAXPROCS`).
- CPU intensive loops in decoders must yield periodically or check for cancel signals to prevent lockups.
