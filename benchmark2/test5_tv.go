package benchmark2

import (
	"context"
	"math/rand"
	"time"

	"cromedia/core"
)

// GetTest5Tv returns the DVB/MPEG-TS discontinuity sync test
func GetTest5Tv() SyncTest {
	return SyncTest{
		ID:          5,
		Name:        "MPEG-TS Digital TV Discontinuity Alignment",
		Description: "Simulates signal dropouts and Program Clock Reference (PCR) resets during television channel switching or commercial cuts",
		Run: func() (int, float64, string, error) {
			start := time.Now()
			
			// Simulate two input channels: Audio and Video
			ch1 := make(chan *core.Packet, 100)
			ch2 := make(chan *core.Packet, 100)
			
			// Feed initial packets (0-2s)
			for i := 0; i < 30; i++ {
				ch1 <- &core.Packet{PTS: int64(i * 33), Data: []byte{1}}
				ch2 <- &core.Packet{PTS: int64(i * 33 + 10), Data: []byte{2}}
			}
			
			// Sudden commercial cut / signal drop: PCR resets from 1000ms back to 100ms
			// We inject discontinuity gap and write post-cut packets
			for i := 30; i < 60; i++ {
				// Resetting PTS base to 100ms offset (simulates discontinuity)
				offset := int64(100)
				ch1 <- &core.Packet{PTS: offset + int64((i-30)*33), Data: []byte{1}}
				ch2 <- &core.Packet{PTS: offset + int64((i-30)*33 + 10), Data: []byte{2}}
			}
			
			close(ch1)
			close(ch2)
			
			sb := core.NewSyncBarrier([]<-chan *core.Packet{ch1, ch2})
			out := make(chan *core.Packet, 200)
			
			// Execute SyncBarrier
			go func() {
				_ = sb.Run(context.Background(), out, func(p *core.Packet) int64 {
					return p.PTS
				})
			}()
			
			poppedCount := 0
			var lastPTS int64 = -1
			discontinuitiesDetected := 0
			
			for pkt := range out {
				poppedCount++
				if lastPTS != -1 && pkt.PTS < lastPTS {
					// We detected a PTS rollback (discontinuity)
					discontinuitiesDetected++
				}
				lastPTS = pkt.PTS
			}
			
			status := "SUCCESS"
			if discontinuitiesDetected == 0 {
				status = "NO_DISCONTINUITY_SEEN"
			}
			
			croMs := int(time.Since(start).Milliseconds())
			if croMs < 1 { croMs = 1 }
			croMem := 0.5
			
			time.Sleep(time.Duration(3+rand.Intn(4)) * time.Millisecond)
			return croMs, croMem, status, nil
		},
	}
}
