package core

import (
	"fmt"
	"time"
)

// MultiTrackCutter handles slicing multiple tracks
type MultiTrackCutter struct {
	Tracks []Track
}

func NewMultiTrackCutter(tracks []Track) *MultiTrackCutter {
	return &MultiTrackCutter{Tracks: tracks}
}

// Cut slices all tracks from startTime to endTime
func (c *MultiTrackCutter) Cut(startTime, endTime time.Duration) ([]Track, error) {
	var cutTracks []Track

	for _, track := range c.Tracks {
		// Calculate time units for this track
		timescale := int64(track.Timescale)
		// Guard against zero timescale
		if timescale == 0 {
			timescale = 1000
		}

		startUnits := int64(startTime.Seconds() * float64(timescale))
		endUnits := int64(endTime.Seconds() * float64(timescale))

		startIdx := -1
		endIdx := -1

		// Find cut points
		for i, s := range track.Samples {
			if s.Time <= startUnits {
				if track.Type == TrackTypeVideo {
					if s.IsKeyframe {
						startIdx = i
					}
				} else {
					startIdx = i // Audio can cut anywhere (ideally)
				}
			}

			if s.Time >= endUnits {
				endIdx = i
				break
			}
		}

		// Fallbacks
		if startIdx == -1 {
			startIdx = 0
		}
		if endIdx == -1 {
			endIdx = len(track.Samples) - 1
		}

		// Slice samples
		if startIdx > endIdx {
			// Empty slice
			fmt.Printf("[Cutter] Track %s: Empty slice (Start %d > End %d)\n", track.Type, startIdx, endIdx)
			continue
		}

		cutSamples := track.Samples[startIdx : endIdx+1]

		// Create new track with cut samples
		cutTrack := track
		cutTrack.Samples = cutSamples
		cutTracks = append(cutTracks, cutTrack)

		fmt.Printf("[Cutter] Track %s (TimeScale %d): Cut %d samples (%d -> %d)\n", track.Type, track.Timescale, len(cutSamples), startIdx, endIdx)
	}

	return cutTracks, nil
}
