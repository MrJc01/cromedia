package demux

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// SniffFormat analyzes the header of a stream to detect its container format.
// Returns one of: "mp4", "webm", "ts", "flv", "ogg", "wav", "mp3", "aac", "flac", or "unknown".
func SniffFormat(r io.ReadSeeker) (string, error) {
	// Preserve current read offset
	origOffset, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return "unknown", err
	}
	defer func() {
		_, _ = r.Seek(origOffset, io.SeekStart)
	}()

	buf := make([]byte, 564) // Large enough to test MPEG-TS 188*3 sync
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "unknown", err
	}
	if n < 4 {
		return "unknown", nil
	}

	// 1. MP4 detection (ftyp box at offset 4)
	if n >= 8 && bytes.Equal(buf[4:8], []byte("ftyp")) {
		return "mp4", nil
	}

	// 2. EBML / WebM / Matroska
	if bytes.HasPrefix(buf, []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		return "webm", nil
	}

	// 3. FLV
	if bytes.HasPrefix(buf, []byte("FLV\x01")) {
		return "flv", nil
	}

	// 4. Ogg
	if bytes.HasPrefix(buf, []byte("OggS")) {
		return "ogg", nil
	}

	// 5. WAV & WebP
	if n >= 12 && bytes.Equal(buf[0:4], []byte("RIFF")) {
		if bytes.Equal(buf[8:12], []byte("WAVE")) {
			return "wav", nil
		}
		if bytes.Equal(buf[8:12], []byte("WEBP")) {
			return "webp", nil
		}
	}

	// 6. FLAC
	if bytes.HasPrefix(buf, []byte("fLaC")) {
		return "flac", nil
	}

	// 7. MPEG-TS (Sync byte 0x47 every 188 bytes)
	if n >= 376 {
		isTS := true
		for i := 0; i < n-188; i += 188 {
			if buf[i] != 0x47 {
				isTS = false
				break
			}
		}
		if isTS && buf[0] == 0x47 {
			return "ts", nil
		}
	}

	// 8. MP3 (ID3v2 tags start with "ID3")
	if bytes.HasPrefix(buf, []byte("ID3")) {
		return "mp3", nil
	}

	// 9. ADTS AAC or MP3 (raw frame headers starting with 0xFF)
	if buf[0] == 0xFF && (buf[1]&0xF0) == 0xF0 {
		// ADTS Layer has bits [2:1] as 00
		if (buf[1] & 0x06) == 0 {
			return "aac", nil
		}
		// MP3 Layer has bits [2:1] != 00
		if (buf[1] & 0x06) != 0 {
			return "mp3", nil
		}
	}

	// Double check generic MP3 sync word (11 bits set: 0xFFE0 mask)
	if buf[0] == 0xFF && (buf[1]&0xE0) == 0xE0 {
		if (buf[1] & 0x06) != 0 {
			return "mp3", nil
		}
	}

	// 10. Subtitle detection (SRT or WebVTT)
	if bytes.Contains(buf[:n], []byte("-->")) {
		if bytes.HasPrefix(buf, []byte("WEBVTT")) {
			return "vtt", nil
		}
		return "srt", nil
	}

	// 11. Annex B (Raw H.264/H.265 starts with 0x000001 or 0x00000001)
	if n >= 4 {
		if buf[0] == 0x00 && buf[1] == 0x00 && buf[2] == 0x01 {
			nalType := buf[3] & 0x1F
			if nalType >= 1 && nalType <= 9 {
				return "annexb", nil
			}
		} else if buf[0] == 0x00 && buf[1] == 0x00 && buf[2] == 0x00 && buf[3] == 0x01 {
			if n >= 5 {
				nalType := buf[4] & 0x1F
				if nalType >= 1 && nalType <= 9 {
					return "annexb", nil
				}
			}
		}
	}

	return "unknown", nil
}

// NewDemuxerFromFormat creates a Demuxer matching the auto-detected format.
func NewDemuxerFromFormat(format string, file *os.File) (Demuxer, error) {
	switch format {
	case "mp4":
		return NewMP4Demuxer(file), nil
	case "webm":
		return NewWebMDemuxer(file), nil
	case "ts":
		return NewTSDemuxer(file), nil
	case "flv":
		return NewFLVDemuxer(file), nil
	case "ogg":
		return NewOggDemuxer(file), nil
	case "wav":
		return NewWAVDemuxer(file), nil
	case "mp3":
		return NewMP3Demuxer(file), nil
	case "aac":
		return NewAACDemuxer(file), nil
	case "flac":
		return NewFLACDemuxer(file), nil
	case "annexb":
		return NewAnnexBDemuxer(file), nil
	case "webp":
		return NewWebPDemuxer(file), nil
	case "srt":
		return NewSRTDemuxer(file), nil
	case "vtt":
		return NewWebVTTDemuxer(file), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}
