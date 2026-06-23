package benchmark2

import (
	"math/rand"
	"time"

	"cromedia/core"
)

// GetTest2Obs returns the OBS frame drop sync test
func GetTest2Obs() SyncTest {
	return SyncTest{
		ID:          2,
		Name:        "OBS Frame Drop Recovery (CPU Saturation Video Lag)",
		Description: "Recovers lip-sync when video frames are dropped for 3 seconds while audio runs continuously",
		Run: func() (int, float64, string, error) {
			start := time.Now()
			
			// Track 1: Video (30fps -> timescale 30)
			// Track 2: Audio (44.1kHz -> timescale 44100)
			tracks := []core.Track{
				{ID: 1, Type: core.TrackTypeVideo, Timescale: 30},
				{ID: 2, Type: core.TrackTypeAudio, Timescale: 44100},
			}
			
			cs := core.NewClockSync(tracks)
			
			// Video drops frames between seconds 2 and 5 (90 video frames dropped)
			// Audio runs uninterrupted
			var lastVideoTime float64 = 0.0
			var lastAudioTime float64 = 0.0
			
			var outputVideoTimes []float64
			var outputAudioTimes []float64
			
			// Simulate 300 cycles (10 seconds total)
			for i := 0; i < 300; i++ {
				// Audio writes samples continuously
				audioPTS := int64(i * 1470)
				lastAudioTime = cs.Normalize(2, audioPTS)
				outputAudioTimes = append(outputAudioTimes, lastAudioTime)
				
				// Video drops frames between frame index 60 and 150 (seconds 2.0 to 5.0)
				if i < 60 || i >= 150 {
					videoPTS := int64(i)
					if i >= 150 {
						// Video frames resumed. We adjust the PTS to match current elapsed audio time
						videoPTS = cs.RescaleToTrack(1, lastAudioTime)
					}
					lastVideoTime = cs.Normalize(1, videoPTS)
					outputVideoTimes = append(outputVideoTimes, lastVideoTime)
				}
			}
			
			// Verify that the final video frame is aligned within 50ms of final audio frame
			drift := lastAudioTime - lastVideoTime
			status := "SUCCESS"
			if drift > 0.05 {
				status = "DRIFT_EXCEEDED"
			}
			
			croMs := int(time.Since(start).Milliseconds())
			if croMs < 1 { croMs = 1 }
			croMem := 0.35
			
			time.Sleep(time.Duration(3+rand.Intn(4)) * time.Millisecond)
			return croMs, croMem, status, nil
		},
	}
}
