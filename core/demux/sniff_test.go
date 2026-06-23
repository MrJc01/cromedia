package demux

import (
	"bytes"
	"io"
	"testing"
)

type mockReadSeeker struct {
	*bytes.Reader
}

func (m *mockReadSeeker) Close() error {
	return nil
}

func TestSniffFormat(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "MP4",
			data:     []byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'm', 'p', '4', '2'},
			expected: "mp4",
		},
		{
			name:     "WebM/MKV",
			data:     []byte{0x1A, 0x45, 0xDF, 0xA3, 0x01, 0x02, 0x03},
			expected: "webm",
		},
		{
			name:     "FLV",
			data:     []byte{'F', 'L', 'V', 0x01, 0x05, 0, 0, 0, 9},
			expected: "flv",
		},
		{
			name:     "Ogg",
			data:     []byte{'O', 'g', 'g', 'S', 0, 0, 0, 0},
			expected: "ogg",
		},
		{
			name:     "WAV",
			data:     []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'A', 'V', 'E'},
			expected: "wav",
		},
		{
			name:     "FLAC",
			data:     []byte{'f', 'L', 'a', 'C', 0, 0, 0, 0},
			expected: "flac",
		},
		{
			name:     "MP3 (ID3)",
			data:     []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0},
			expected: "mp3",
		},
		{
			name:     "MP3 (Raw Frame)",
			data:     []byte{0xFF, 0xFB, 0x90, 0x00},
			expected: "mp3",
		},
		{
			name:     "ADTS AAC",
			data:     []byte{0xFF, 0xF1, 0x50, 0x80},
			expected: "aac",
		},
		{
			name:     "MPEG-TS",
			data: func() []byte {
				// Create TS packets of 188 bytes starting with 0x47
				buf := make([]byte, 564)
				buf[0] = 0x47
				buf[188] = 0x47
				buf[376] = 0x47
				return buf
			}(),
			expected: "ts",
		},
		{
			name:     "Unknown",
			data:     []byte{0x00, 0x11, 0x22, 0x33, 0x44},
			expected: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rs := &mockReadSeeker{Reader: bytes.NewReader(tc.data)}
			format, err := SniffFormat(rs)
			if err != nil {
				t.Fatalf("SniffFormat returned unexpected error: %v", err)
			}
			if format != tc.expected {
				t.Errorf("Expected format '%s', got '%s'", tc.expected, format)
			}
			// Verify read pointer is preserved
			pos, err := rs.Seek(0, io.SeekCurrent)
			if err != nil {
				t.Fatalf("Seek failed: %v", err)
			}
			if pos != 0 {
				t.Errorf("Expected read offset to be reset to 0, got %d", pos)
			}
		})
	}
}
