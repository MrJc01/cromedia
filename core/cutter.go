package core

import (
	"fmt"
	"math"
	"time"
)

// MultiTrackCutter handles slicing multiple tracks
type MultiTrackCutter struct {
	Tracks []Track
}

func NewMultiTrackCutter(tracks []Track) *MultiTrackCutter {
	return &MultiTrackCutter{Tracks: tracks}
}

// CutWithReport slices all tracks and returns cut reports with keyframe delta info
func (c *MultiTrackCutter) CutWithReport(startTime, endTime time.Duration) ([]Track, []CutReport, error) {
	var cutTracks []Track
	var reports []CutReport

	for _, track := range c.Tracks {
		timescale := int64(track.Timescale)
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
					startIdx = i
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
			fmt.Printf("[Cutter] Track %s: Empty slice (Start %d > End %d)\n", track.Type, startIdx, endIdx)
			continue
		}

		cutSamples := track.Samples[startIdx : endIdx+1]

		// Calculate actual times for the report
		actualStartSec := float64(track.Samples[startIdx].Time) / float64(timescale)
		actualEndSec := float64(track.Samples[endIdx].Time) / float64(timescale)
		requestedStartSec := startTime.Seconds()
		requestedEndSec := endTime.Seconds()

		deltaStartMs := (actualStartSec - requestedStartSec) * 1000.0
		deltaEndMs := (actualEndSec - requestedEndSec) * 1000.0

		report := CutReport{
			TrackType:       track.Type,
			RequestedStart:  requestedStartSec,
			ActualStart:     actualStartSec,
			RequestedEnd:    requestedEndSec,
			ActualEnd:       actualEndSec,
			DeltaStartMs:    deltaStartMs,
			DeltaEndMs:      deltaEndMs,
			SamplesIncluded: len(cutSamples),
		}
		reports = append(reports, report)

		// Also slice CTSOffsets if present
		cutTrack := track
		cutTrack.Samples = cutSamples
		if len(track.CTSOffsets) > 0 && endIdx < len(track.CTSOffsets) {
			cutTrack.CTSOffsets = track.CTSOffsets[startIdx : endIdx+1]
		} else if len(track.CTSOffsets) > 0 {
			// Partial: take what we can
			end := endIdx + 1
			if end > len(track.CTSOffsets) {
				end = len(track.CTSOffsets)
			}
			if startIdx < end {
				cutTrack.CTSOffsets = track.CTSOffsets[startIdx:end]
			}
		}
		cutTracks = append(cutTracks, cutTrack)

		// Print report with keyframe warning
		if track.Type == TrackTypeVideo && math.Abs(deltaStartMs) > 1.0 {
			fmt.Printf("[Cutter] ⚠️  Track %s: Corte ajustado para keyframe!\n", track.Type)
			fmt.Printf("         Solicitado: %.3fs → Real: %.3fs (Δ %.1fms)\n", requestedStartSec, actualStartSec, deltaStartMs)
		}
		fmt.Printf("[Cutter] Track %s (TS %d): %d samples [%.3fs → %.3fs] (Δstart=%.1fms, Δend=%.1fms)\n",
			track.Type, track.Timescale, len(cutSamples),
			actualStartSec, actualEndSec, deltaStartMs, deltaEndMs)
	}

	return cutTracks, reports, nil
}

// Cut is the backward-compatible version without reports
func (c *MultiTrackCutter) Cut(startTime, endTime time.Duration) ([]Track, error) {
	tracks, _, err := c.CutWithReport(startTime, endTime)
	return tracks, err
}
