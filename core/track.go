package core

// TrackType enum
type TrackType string

const (
	TrackTypeVideo TrackType = "vide"
	TrackTypeAudio TrackType = "soun"
	TrackTypeHint  TrackType = "hint"
	TrackTypeMeta  TrackType = "meta"
)

// EditListEntry represents a single entry in an Edit List (elst)
type EditListEntry struct {
	SegmentDuration uint64 // Duration of this edit in movie timescale
	MediaTime       int64  // Starting time in media timescale (-1 = empty edit/dwell)
	MediaRateInt    int16  // Usually 1
	MediaRateFrac   int16  // Usually 0
}

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

	// Edit List (edts/elst) — Sync correction
	// MediaTimeOffset is the initial delay in media timescale units.
	// Positive = skip N units at start of media. Used for A/V sync.
	EditList        []EditListEntry
	MediaTimeOffset int64 // Computed from first edit: the initial presentation offset
}

// InterleavedSample is used for interleaved mdat writing
type InterleavedSample struct {
	TrackIndex  int
	SampleIndex int
	TimeSeconds float64 // Normalized time for cross-track ordering
	Sample      Sample
}

// CutReport contains metadata about the cut operation for user feedback
type CutReport struct {
	TrackType       TrackType
	RequestedStart  float64 // Seconds
	ActualStart     float64 // Seconds (snapped to keyframe)
	RequestedEnd    float64 // Seconds
	ActualEnd       float64 // Seconds
	DeltaStartMs    float64 // Difference in milliseconds
	DeltaEndMs      float64 // Difference in milliseconds
	SamplesIncluded int
}
