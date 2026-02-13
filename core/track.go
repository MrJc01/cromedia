package core

// TrackType enum
type TrackType string

const (
	TrackTypeVideo TrackType = "vide"
	TrackTypeAudio TrackType = "soun"
	TrackTypeHint  TrackType = "hint"
	TrackTypeMeta  TrackType = "meta"
)

// Track represents a single media track (Video or Audio)
type Track struct {
	ID        int
	Type      TrackType
	Timescale uint32
	Duration  uint64
	Samples   []Sample

	// Metadata Payloads (Raw Bytes excluding header)
	Stsd        []byte // Sample Description (Codec Config)
	Hdlr        []byte // Handler Reference
	MediaHeader []byte // vmhd (Video) or smhd (Audio)
	Tkhd        []byte // Track Header

	// Video Specific
	Width  uint32
	Height uint32

	// Audio Specific
	Volume uint16

	// B-Frame Support: Composition Time Offsets (ctts)
	// Per-sample CTS offsets. If empty, PTS == DTS (no B-Frames).
	CTSOffsets []int32

	// Codec Detection
	CodecTag string // "avc1", "hev1", "mp4a", etc.
}

// InterleavedSample is used for interleaved mdat writing
type InterleavedSample struct {
	TrackIndex  int
	SampleIndex int
	TimeSeconds float64 // Normalized time for cross-track ordering
	Sample      Sample
}
