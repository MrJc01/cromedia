# CroMedia Image & Sequence Modules 📷

This guide explains how to use CroMedia's native static image module and the sequence demuxer/muxer pipelines.

---

## 1. Converting between Images and VideoFrames
CroMedia provides high-performance conversion functions between `image.Image` and `core.VideoFrame`.

```go
import "cromedia/core/image"

// Convert image.Image to core.VideoFrame (allocates from BufferPool)
frame, err := image.ConvertToVideoFrame(myImage)

// Convert core.VideoFrame (RGBA) to standard image.Image
img, err := image.ConvertToImage(frame)
```

## 2. Reading and Writing Image Formats
Supported formats out of the box include JPEG, PNG, BMP, WebP, and TIFF.

```go
// Decode image from reader (with magic bytes sniffing)
img, format, err := image.DecodeImage(reader)

// Encode image to writer with compression quality (1-100)
err := image.EncodeImage(writer, img, "png", 90)
```

## 3. Parallel Image Sequence Demuxing & Muxing
The `SequenceDemuxer` reads an ordered sequence of images (or animated GIFs) and streams them as `VideoFrames`, utilizing `core.GlobalWorkerPool` to prefetch and decode frames in parallel.

```go
// Create a new sequence demuxer matching a glob or printf pattern
demuxer, err := image.NewSequenceDemuxer("path/to/frames_%04d.jpg", 30.0)
defer demuxer.Close()

tracks, err := demuxer.Probe()
// Read packets...
```

The `SequenceMuxer` outputs video packets or frames to individual files or writes them into an animated GIF:

```go
muxer, err := image.NewSequenceMuxer("output/out_%04d.png")
defer muxer.Close()

// Write frames
err := muxer.WriteFrame(myFrame)
```
