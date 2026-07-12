package pipeline

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"io"
	"io/ioutil"
	"os"

	"cromedia/core"
	coreImage "cromedia/core/image"
	"cromedia/core/mux"
)

// RenderScenePipeline takes a channel of frames (image.Image or JPEG []byte),
// encodes them using the provided video encoder, and outputs them into a standard MP4 file.
// If an audioTrack and audioFile are provided, they are muxed alongside the video.
func RenderScenePipeline(
	outputPath string,
	width, height int,
	fps int,
	encoder core.VideoEncoder,
	frames <-chan interface{},
	audioTrack *core.Track,
	audioFile *os.File,
) error {
	if encoder == nil {
		return errors.New("video encoder cannot be nil")
	}

	// 1. Create a temporary file to store encoded H.264 packets
	tmpFile, err := ioutil.TempFile("", "render_video_*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary video packet cache: %w", err)
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()

	var videoSamples []core.Sample
	videoTimescale := uint32(90000)
	videoFrameDuration := int64(90000 / fps)
	var videoPTS int64 = 0

	var sps []byte
	var pps []byte

	// Helper to write packets to the temp cache and record sample metadata
	writePacket := func(pkt *core.Packet) error {
		if pkt == nil {
			return nil
		}

		offset, err := tmpFile.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}

		_, err = tmpFile.Write(pkt.Data)
		if err != nil {
			return err
		}

		videoSamples = append(videoSamples, core.Sample{
			ID:         len(videoSamples) + 1,
			IsKeyframe: pkt.IsKeyframe,
			Offset:     offset,
			Size:       int64(len(pkt.Data)),
			Time:       videoPTS,
			Duration:   videoFrameDuration,
		})

		// Try to parse Annex B NAL units to extract SPS/PPS if not already done
		if len(sps) == 0 || len(pps) == 0 {
			nals := core.ParseAnnexBNalUnits(pkt.Data)
			for _, nal := range nals {
				if len(nal) > 0 {
					nalType := nal[0] & 0x1F
					if nalType == 7 { // SPS
						sps = append([]byte{}, nal...)
					} else if nalType == 8 { // PPS
						pps = append([]byte{}, nal...)
					}
				}
			}
		}

		videoPTS += videoFrameDuration
		return nil
	}

	// 2. Read frames from channel, convert/color-space and encode
	for item := range frames {
		var img image.Image
		switch f := item.(type) {
		case []byte:
			var decodeErr error
			img, _, decodeErr = coreImage.DecodeImage(bytes.NewReader(f))
			if decodeErr != nil {
				return fmt.Errorf("failed to decode JPEG bytes: %w", decodeErr)
			}
		case image.Image:
			img = f
		default:
			return fmt.Errorf("unsupported frame type: %T", item)
		}

		// Convert to VideoFrame (RGBA)
		rgbaFrame, err := coreImage.ConvertToVideoFrame(img)
		if err != nil {
			return fmt.Errorf("failed to convert image to video frame: %w", err)
		}

		// Convert RGBA to YUV420p for standard H.264 compression
		yuvData := coreImage.ConvertRGBAToYUV420p(rgbaFrame.Width, rgbaFrame.Height, rgbaFrame.Data)
		core.GlobalPut(rgbaFrame.Data) // Return RGBA buffer to the pool immediately

		yuvFrame := &core.VideoFrame{
			Width:  rgbaFrame.Width,
			Height: rgbaFrame.Height,
			Format: core.PixelFormatYUV420P,
			Data:   yuvData,
		}

		// Encode the YUV frame
		pkt, err := encoder.Encode(yuvFrame)
		core.GlobalPut(yuvData) // Return YUV buffer to the pool
		if err != nil {
			return fmt.Errorf("failed to encode frame: %w", err)
		}

		if pkt != nil {
			if err := writePacket(pkt); err != nil {
				return fmt.Errorf("failed to process encoded packet: %w", err)
			}
		}
	}

	// 3. Flush the encoder to retrieve any remaining buffered packets
	for {
		pkt, err := encoder.Encode(nil)
		if err != nil {
			return fmt.Errorf("failed to flush encoder: %w", err)
		}
		if pkt == nil {
			break
		}
		if err := writePacket(pkt); err != nil {
			return fmt.Errorf("failed to process flushed packet: %w", err)
		}
	}

	if len(videoSamples) == 0 {
		return errors.New("no frames were successfully encoded")
	}

	// 4. Fallback default SPS/PPS if they were not generated in the stream
	if len(sps) == 0 {
		sps = []byte{0x67, 0x64, 0x00, 0x1f, 0xac, 0xd9, 0x40, 0x50, 0x05, 0xbb, 0x01, 0x10, 0x00, 0x00, 0x03, 0x00, 0x10, 0x00, 0x00, 0x03, 0x03, 0x20, 0xf1, 0x83, 0x19, 0x60}
	}
	if len(pps) == 0 {
		pps = []byte{0x68, 0xeb, 0xec, 0xb2, 0x2c}
	}

	// 5. Construct the Video Track
	videoTrack := core.Track{
		ID:          1,
		Type:        core.TrackTypeVideo,
		Timescale:   videoTimescale,
		Duration:    uint64(videoPTS),
		Width:       uint32(width),
		Height:      uint32(height),
		CodecTag:    "avc1",
		Samples:     videoSamples,
		Hdlr:        mux.DefaultVideoHdlr(),
		MediaHeader: mux.DefaultVideoMediaHeader(),
		Stsd:        mux.MakeH264Stsd(width, height, sps, pps),
	}

	// Prepare track list for multiplexing
	var tracks []core.Track
	tracks = append(tracks, videoTrack)

	if audioTrack != nil && audioFile != nil {
		aTrack := *audioTrack
		aTrack.ID = 2
		if len(aTrack.Hdlr) == 0 {
			aTrack.Hdlr = mux.DefaultAudioHdlr()
		}
		if len(aTrack.MediaHeader) == 0 {
			aTrack.MediaHeader = mux.DefaultAudioMediaHeader()
		}
		if len(aTrack.Stsd) == 0 {
			aTrack.Stsd = mux.MakeAudioStsd(aTrack.CodecTag, int(aTrack.Timescale), 2)
		}
		tracks = append(tracks, aTrack)
	}

	// 6. Create the final output MP4 file and write headers
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create final output file: %w", err)
	}
	defer out.Close()

	muxer := mux.NewMP4Muxer(out)
	err = muxer.WriteHeader(tracks)
	if err != nil {
		return fmt.Errorf("failed to write final MP4 container header: %w", err)
	}

	// 7. Mux the tracks by copying sample data in interleaved order
	// This ensures proper hardware synchronization on playback
	type trackSourceInfo struct {
		file *os.File
	}
	sources := make(map[int]trackSourceInfo)
	sources[0] = trackSourceInfo{file: tmpFile}
	if audioTrack != nil {
		sources[1] = trackSourceInfo{file: audioFile}
	}

	interleaved := mux.BuildInterleavedOrder(tracks)

	// Pre-allocate a 64KB block copy buffer from the pool
	bufSize := 65536
	copyBuf := core.GlobalGet(bufSize)
	defer core.GlobalPut(copyBuf)

	for _, is := range interleaved {
		src := sources[is.TrackIndex]
		origSample := tracks[is.TrackIndex].Samples[is.SampleIndex]

		_, err = src.file.Seek(origSample.Offset, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek to sample %d in source track %d: %w", is.SampleIndex, is.TrackIndex, err)
		}

		remaining := origSample.Size
		for remaining > 0 {
			toRead := int64(bufSize)
			if remaining < toRead {
				toRead = remaining
			}

			_, err = io.ReadFull(src.file, copyBuf[:toRead])
			if err != nil {
				return fmt.Errorf("failed to read sample data from track source: %w", err)
			}

			pkt := &core.Packet{Data: copyBuf[:toRead]}
			err = muxer.WritePacket(pkt)
			if err != nil {
				return fmt.Errorf("failed to write packet to final container: %w", err)
			}
			remaining -= toRead
		}
	}

	err = muxer.WriteTrailer()
	if err != nil {
		return fmt.Errorf("failed to write trailer: %w", err)
	}

	return nil
}
