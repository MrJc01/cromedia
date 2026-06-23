package core

import "math"

// Rescale converts a timestamp from one timescale to another.
func Rescale(timestamp int64, fromTimescale, toTimescale uint32) int64 {
	if fromTimescale == 0 || fromTimescale == toTimescale {
		return timestamp
	}
	return int64(math.Round(float64(timestamp) * float64(toTimescale) / float64(fromTimescale)))
}

// ClockSync manages timescale mappings across multiple tracks to ensure clean presentation sync.
type ClockSync struct {
	tracks map[int]Track
}

// NewClockSync initializes a clock synchronizer with the given tracks.
func NewClockSync(tracks []Track) *ClockSync {
	trackMap := make(map[int]Track)
	for _, t := range tracks {
		trackMap[t.ID] = t
	}
	return &ClockSync{tracks: trackMap}
}

// Normalize converts a track-specific timestamp (e.g. PTS or DTS) to normal seconds.
func (cs *ClockSync) Normalize(trackID int, timestamp int64) float64 {
	t, ok := cs.tracks[trackID]
	if !ok || t.Timescale == 0 {
		return float64(timestamp) / 1000.0
	}
	return float64(timestamp) / float64(t.Timescale)
}

// RescaleToTrack converts standard time units (e.g. seconds) back to track-specific units.
func (cs *ClockSync) RescaleToTrack(trackID int, seconds float64) int64 {
	t, ok := cs.tracks[trackID]
	if !ok || t.Timescale == 0 {
		return int64(seconds * 1000.0)
	}
	return int64(math.Round(seconds * float64(t.Timescale)))
}

// Compare compares timestamps across two different tracks. Returns:
// -1 if t1 < t2, 0 if t1 == t2, 1 if t1 > t2.
func (cs *ClockSync) Compare(trackID1 int, t1 int64, trackID2 int, t2 int64) int {
	sec1 := cs.Normalize(trackID1, t1)
	sec2 := cs.Normalize(trackID2, t2)

	const epsilon = 1e-9 // Nanosecond precision
	if math.Abs(sec1-sec2) < epsilon {
		return 0
	}
	if sec1 < sec2 {
		return -1
	}
	return 1
}
