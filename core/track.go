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

	// Audio Specific (TODO: Parse explicit values if needed, for now just payload)
	Volume uint16
}
