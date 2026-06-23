package pipeline

import (
	"errors"
	"fmt"
	"io"
	"os"

	"cromedia/core"
	"cromedia/core/demux"
	"cromedia/core/filters/audio"
	"cromedia/core/mux"
)

// makeAudioSpecificConfig generates the AudioSpecificConfig byte slice for AAC-LC.
func makeAudioSpecificConfig(sampleRate int, channels int) []byte {
	srIndex := 4 // Default 44100
	switch sampleRate {
	case 96000: srIndex = 0
	case 88200: srIndex = 1
	case 64000: srIndex = 2
	case 48000: srIndex = 3
	case 44100: srIndex = 4
	case 32000: srIndex = 5
	case 24000: srIndex = 6
	case 22050: srIndex = 7
	case 16000: srIndex = 8
	case 12000: srIndex = 9
	case 11025: srIndex = 10
	case 8000:  srIndex = 11
	case 7350:  srIndex = 12
	}

	b0 := byte(2<<3) | byte(srIndex>>1)
	b1 := byte((srIndex&1)<<7) | byte((channels&15)<<3)
	return []byte{b0, b1}
}

// MixAudioTracks extracts original audio from videoMP4 (AAC), extracts audio from soundtrackMP3 (MP3),
// overlays them using AudioMixer with a volume of 0.3 on the soundtrack, encodes the result to AAC,
// and muxes it alongside the stream-copied original video track into outputMP4.
func MixAudioTracks(videoMP4, soundtrackMP3, outputMP4 string) error {
	videoFile, err := os.Open(videoMP4)
	if err != nil {
		return fmt.Errorf("failed to open video file: %w", err)
	}
	defer videoFile.Close()

	demuxer := demux.NewMP4Demuxer(videoFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		return fmt.Errorf("failed to probe video file: %w", err)
	}

	var videoTrack *core.Track
	var voiceTrack *core.Track
	voiceTrackIndex := -1

	for i := range tracks {
		if tracks[i].Type == core.TrackTypeVideo {
			videoTrack = &tracks[i]
		} else if tracks[i].Type == core.TrackTypeAudio {
			voiceTrack = &tracks[i]
			voiceTrackIndex = i
		}
	}

	if videoTrack == nil {
		return errors.New("no video track found in input MP4")
	}

	var voiceFrame *core.AudioFrame
	var voiceFrames []*core.AudioFrame

	if voiceTrack != nil {
		aacDec := &core.SimAACDecoder{}
		if len(voiceTrack.CodecPrivate) > 0 {
			_ = aacDec.Init(voiceTrack.CodecPrivate)
		}
		defer aacDec.Close()

		for {
			pkt, err := demuxer.ReadPacket()
			if err != nil {
				if err == io.EOF {
					break
				}
				return fmt.Errorf("failed to read packet: %w", err)
			}

			if pkt.StreamIndex == voiceTrackIndex {
				f, err := aacDec.Decode(pkt)
				core.GlobalPut(pkt.Data)
				if err != nil {
					return fmt.Errorf("failed to decode AAC packet: %w", err)
				}
				if f != nil {
					voiceFrames = append(voiceFrames, f)
				}
			} else {
				core.GlobalPut(pkt.Data)
			}
		}

		if len(voiceFrames) > 0 {
			totalSamples := 0
			for _, f := range voiceFrames {
				totalSamples += len(f.Data)
			}
			voiceData := make([]float32, totalSamples)
			offset := 0
			for _, f := range voiceFrames {
				copy(voiceData[offset:], f.Data)
				offset += len(f.Data)
			}
			voiceFrame = &core.AudioFrame{
				Channels:   voiceFrames[0].Channels,
				SampleRate: voiceFrames[0].SampleRate,
				Data:       voiceData,
			}
		}
	}

	// Decode soundtrack MP3
	musicFrame, err := core.DecodeMP3File(soundtrackMP3)
	if err != nil {
		return fmt.Errorf("failed to decode soundtrack MP3: %w", err)
	}

	// Mix voice and music
	mixedFrame := audio.MixAudioFrames(voiceFrame, musicFrame, 1.0, 0.3, true)
	if mixedFrame == nil {
		return errors.New("mixed audio frame is nil")
	}

	targetChannels := mixedFrame.Channels
	targetSampleRate := mixedFrame.SampleRate

	// Encode to AAC
	aacEnc := &core.SimAACEncoder{}
	defer aacEnc.Close()

	tmpAudioFile, err := os.CreateTemp("", "mix_audio_*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary audio file: %w", err)
	}
	defer func() {
		tmpAudioFile.Close()
		os.Remove(tmpAudioFile.Name())
	}()

	var audioSamples []core.Sample
	var audioPTS int64 = 0
	audioFrameDuration := int64(1024)

	writeAudioPacket := func(pkt *core.Packet) error {
		if pkt == nil {
			return nil
		}
		offset, err := tmpAudioFile.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		_, err = tmpAudioFile.Write(pkt.Data)
		if err != nil {
			return err
		}
		audioSamples = append(audioSamples, core.Sample{
			ID:         len(audioSamples) + 1,
			IsKeyframe: true,
			Offset:     offset,
			Size:       int64(len(pkt.Data)),
			Time:       audioPTS,
			Duration:   audioFrameDuration,
		})
		audioPTS += audioFrameDuration
		return nil
	}

	// Chunk mixed frame to prevent blocking in aacEnc.outChan
	chunkSize := 1024 * targetChannels
	for i := 0; i < len(mixedFrame.Data); i += chunkSize {
		end := i + chunkSize
		if end > len(mixedFrame.Data) {
			end = len(mixedFrame.Data)
		}
		chunkData := mixedFrame.Data[i:end]

		chunkFrame := &core.AudioFrame{
			Channels:   targetChannels,
			SampleRate: targetSampleRate,
			Data:       chunkData,
		}

		pkt, err := aacEnc.Encode(chunkFrame)
		if err != nil {
			return fmt.Errorf("failed to encode audio chunk: %w", err)
		}
		if pkt != nil {
			if err := writeAudioPacket(pkt); err != nil {
				return err
			}
		}
	}

	// Flush audio encoder
	for {
		pkt, err := aacEnc.Encode(nil)
		if err != nil {
			return fmt.Errorf("failed to flush audio encoder: %w", err)
		}
		if pkt == nil {
			break
		}
		if err := writeAudioPacket(pkt); err != nil {
			return err
		}
	}

	if len(audioSamples) == 0 {
		return errors.New("no audio frames were successfully encoded")
	}

	// Construct final audio track
	optAudioTrack := core.Track{
		ID:          2,
		Type:        core.TrackTypeAudio,
		Timescale:   uint32(targetSampleRate),
		Duration:    uint64(audioPTS),
		CodecTag:    "mp4a",
		Samples:     audioSamples,
		Hdlr:        mux.DefaultAudioHdlr(),
		MediaHeader: mux.DefaultAudioMediaHeader(),
		Stsd:        mux.MakeAACStsd(targetSampleRate, targetChannels, makeAudioSpecificConfig(targetSampleRate, targetChannels)),
	}

	// Video track is stream-copied directly
	optVideoTrack := *videoTrack
	optVideoTrack.ID = 1

	var outTracks []core.Track
	outTracks = append(outTracks, optVideoTrack)
	outTracks = append(outTracks, optAudioTrack)

	outFile, err := os.Create(outputMP4)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	muxer := mux.NewMP4Muxer(outFile)
	if err := muxer.WriteHeader(outTracks); err != nil {
		return fmt.Errorf("failed to write header to final container: %w", err)
	}

	sources := make(map[int]io.ReadSeeker)
	
	// Track 0 is video (from original video file)
	videoSourceFile, err := os.Open(videoMP4)
	if err != nil {
		return fmt.Errorf("failed to open video source file for copy: %w", err)
	}
	defer videoSourceFile.Close()
	sources[0] = videoSourceFile

	// Track 1 is audio (from temp audio file)
	sources[1] = tmpAudioFile

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
