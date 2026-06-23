package pipeline

import (
	"errors"
	"fmt"
	"io"
	"os"

	"cromedia/core"
	"cromedia/core/demux"
	coreImage "cromedia/core/image"
	"cromedia/core/mux"
)

// OptimizeVideoAsset opens an MP4 file, decodes the video using H.264, scales it to 1920x1080,
// and encodes it forcing all-keyframes (keyint=1) before muxing the final output MP4.
// If the input asset has audio, the audio is preserved via interleaved stream copy.
func OptimizeVideoAsset(inputPath, outputPath string) error {
	inFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inFile.Close()

	demuxer := demux.NewMP4Demuxer(inFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		return fmt.Errorf("failed to probe input file: %w", err)
	}

	var videoTrack *core.Track
	var audioTrack *core.Track
	videoTrackIndex := -1

	for i := range tracks {
		if tracks[i].Type == core.TrackTypeVideo {
			videoTrack = &tracks[i]
			videoTrackIndex = i
		} else if tracks[i].Type == core.TrackTypeAudio {
			audioTrack = &tracks[i]
		}
	}

	if videoTrack == nil {
		return errors.New("no video track found in input file")
	}

	// Calculate FPS (frame rate)
	fps := 30
	if len(videoTrack.Samples) > 0 && videoTrack.Samples[0].Duration > 0 {
		fps = int(float64(videoTrack.Timescale) / float64(videoTrack.Samples[0].Duration))
	}
	if fps <= 0 {
		fps = 30
	}

	// Instantiate the decoder and encoder
	decoder := &core.SimH264Decoder{}
	defer decoder.Close()

	encoder := &core.SimH264Encoder{
		KeyintMax: 1, // Force all-keyframes (keyint=1)
	}

	tmpVideoFile, err := os.CreateTemp("", "optimize_video_*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary video file: %w", err)
	}
	defer func() {
		tmpVideoFile.Close()
		os.Remove(tmpVideoFile.Name())
	}()

	var videoSamples []core.Sample
	var videoPTS int64 = 0
	videoTimescale := uint32(90000)
	if videoTrack.Timescale > 0 {
		videoTimescale = videoTrack.Timescale
	}
	videoFrameDuration := int64(videoTimescale) / int64(fps)
	if videoFrameDuration <= 0 {
		videoFrameDuration = 3000
	}

	var sps []byte
	var pps []byte

	writeVideoPacket := func(pkt *core.Packet) error {
		if pkt == nil {
			return nil
		}
		offset, err := tmpVideoFile.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		_, err = tmpVideoFile.Write(pkt.Data)
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

		if len(sps) == 0 || len(pps) == 0 {
			nals := core.ParseAnnexBNalUnits(pkt.Data)
			for _, nal := range nals {
				if len(nal) > 0 {
					nalType := nal[0] & 0x1F
					if nalType == 7 {
						sps = append([]byte{}, nal...)
					} else if nalType == 8 {
						pps = append([]byte{}, nal...)
					}
				}
			}
		}
		videoPTS += videoFrameDuration
		return nil
	}

	scaler := &core.ScaleFilter{
		TargetW:  1920,
		TargetH:  1080,
		Bilinear: true,
	}

	// Read packets and process
	for {
		pkt, err := demuxer.ReadPacket()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read packet: %w", err)
		}

		if pkt.StreamIndex == videoTrackIndex {
			frame, err := decoder.Decode(pkt)
			core.GlobalPut(pkt.Data) // Release demuxed packet data back to pool
			if err != nil {
				return fmt.Errorf("failed to decode video frame: %w", err)
			}
			if frame == nil {
				continue
			}

			// Convert to RGBA
			var rgbaData []byte
			if frame.Format == core.PixelFormatYUV420P {
				rgbaData = coreImage.ConvertYUV420pToRGBA(frame.Width, frame.Height, frame.Data)
				core.GlobalPut(frame.Data) // Release YUV data back to pool
			} else if frame.Format == core.PixelFormatRGBA {
				rgbaData = frame.Data
			} else {
				return fmt.Errorf("unsupported input format: %s", frame.Format)
			}

			rgbaFrame := &core.VideoFrame{
				Width:  frame.Width,
				Height: frame.Height,
				Format: core.PixelFormatRGBA,
				Data:   rgbaData,
			}

			// Resize to 1920x1080
			scaledFrame, err := scaler.Process(rgbaFrame)
			core.GlobalPut(rgbaFrame.Data) // Return RGBA data to pool
			if err != nil {
				return fmt.Errorf("failed to scale frame: %w", err)
			}

			// Encode
			outPkt, err := encoder.Encode(scaledFrame)
			if err != nil {
				return fmt.Errorf("failed to encode frame: %w", err)
			}
			if outPkt != nil {
				if err := writeVideoPacket(outPkt); err != nil {
					return err
				}
			}
		} else {
			// Not a video packet, just release its data
			core.GlobalPut(pkt.Data)
		}
	}

	// Flush video encoder
	for {
		outPkt, err := encoder.Encode(nil)
		if err != nil {
			return fmt.Errorf("failed to flush video encoder: %w", err)
		}
		if outPkt == nil {
			break
		}
		if err := writeVideoPacket(outPkt); err != nil {
			return err
		}
	}

	encoder.Close()

	if len(videoSamples) == 0 {
		return errors.New("no video frames were successfully processed")
	}

	// Default SPS/PPS if not parsed in the stream
	if len(sps) == 0 {
		sps = []byte{0x67, 0x64, 0x00, 0x1f, 0xac, 0xd9, 0x40, 0x50, 0x05, 0xbb, 0x01, 0x10, 0x00, 0x00, 0x03, 0x00, 0x10, 0x00, 0x00, 0x03, 0x03, 0x20, 0xf1, 0x83, 0x19, 0x60}
	}
	if len(pps) == 0 {
		pps = []byte{0x68, 0xeb, 0xec, 0xb2, 0x2c}
	}

	// Construct final video track
	optVideoTrack := core.Track{
		ID:          1,
		Type:        core.TrackTypeVideo,
		Timescale:   videoTimescale,
		Duration:    uint64(videoPTS),
		Width:       1920,
		Height:      1080,
		CodecTag:    "avc1",
		Samples:     videoSamples,
		Hdlr:        mux.DefaultVideoHdlr(),
		MediaHeader: mux.DefaultVideoMediaHeader(),
		Stsd:        mux.MakeH264Stsd(1920, 1080, sps, pps),
	}

	var outTracks []core.Track
	outTracks = append(outTracks, optVideoTrack)

	if audioTrack != nil {
		optAudioTrack := *audioTrack
		optAudioTrack.ID = 2
		outTracks = append(outTracks, optAudioTrack)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	muxer := mux.NewMP4Muxer(outFile)
	if err := muxer.WriteHeader(outTracks); err != nil {
		return fmt.Errorf("failed to write header to final container: %w", err)
	}

	sources := make(map[int]io.ReadSeeker)
	sources[0] = tmpVideoFile

	if audioTrack != nil {
		audioFile, err := os.Open(inputPath)
		if err != nil {
			return fmt.Errorf("failed to open input file for audio stream copy: %w", err)
		}
		defer audioFile.Close()
		sources[1] = audioFile
	}

	interleaved := mux.BuildInterleavedOrder(outTracks)
	bufSize := 65536
	copyBuf := core.GlobalGet(bufSize)
	defer core.GlobalPut(copyBuf)

	for _, is := range interleaved {
		src := sources[is.TrackIndex]
		origSample := outTracks[is.TrackIndex].Samples[is.SampleIndex]

		_, err = src.Seek(origSample.Offset, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek to offset %d: %w", origSample.Offset, err)
		}

		remaining := origSample.Size
		for remaining > 0 {
			toRead := int64(bufSize)
			if remaining < toRead {
				toRead = remaining
			}

			_, err = io.ReadFull(src, copyBuf[:toRead])
			if err != nil {
				return fmt.Errorf("failed to read block: %w", err)
			}

			pkt := &core.Packet{Data: copyBuf[:toRead]}
			if err := muxer.WritePacket(pkt); err != nil {
				return fmt.Errorf("failed to write packet: %w", err)
			}
			remaining -= toRead
		}
	}

	return muxer.WriteTrailer()
}
