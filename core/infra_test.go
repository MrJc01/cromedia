package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestPipelineContext(t *testing.T) {
	ctx := NewPipelineContext(context.Background())

	// Stage timing
	finish := ctx.StartStage("demux")
	time.Sleep(50 * time.Millisecond)
	finish()

	_, _, _, latencies, _, _ := ctx.GetTelemetry()

	if latencies["demux"] < 40*time.Millisecond {
		t.Errorf("Expected demux stage latency to be measured (>40ms), got %v", latencies["demux"])
	}

	// Packet counters
	ctx.AddPacket("demux", 10)
	ctx.AddPacket("demux", 5)

	_, _, _, _, packets, _ := ctx.GetTelemetry()
	if packets["demux"] != 15 {
		t.Errorf("Expected packet count to be 15, got %d", packets["demux"])
	}

	// Error logging
	testErr := errors.New("test error")
	ctx.RecordError(testErr)
	_, _, _, _, _, errs := ctx.GetTelemetry()
	if len(errs) != 1 || errs[0] != testErr {
		t.Errorf("Expected 1 error matching testErr, got %v", errs)
	}

	// Panic recovery
	func() {
		defer ctx.RecoverPanic(nil)
		panic("boom")
	}()
	_, _, _, _, _, errs = ctx.GetTelemetry()
	if len(errs) != 2 {
		t.Errorf("Expected 2 errors total after panic recovery, got %d", len(errs))
	}

	// Safe print
	ctx.PrintReport()
}

func TestEventBus(t *testing.T) {
	eb := GetEventBus()
	var wg sync.WaitGroup
	wg.Add(1)

	var receivedEvent Event
	eb.Subscribe("progress", func(ev Event) {
		receivedEvent = ev
		wg.Done()
	})

	eb.Publish("progress", "50%")
	wg.Wait()

	if receivedEvent.Topic != "progress" || receivedEvent.Data.(string) != "50%" {
		t.Errorf("Expected progress 50%% event, got topic %s data %v", receivedEvent.Topic, receivedEvent.Data)
	}
}

func TestClockSync(t *testing.T) {
	// Rescale helper
	res := Rescale(1000, 1000, 90000)
	if res != 90000 {
		t.Errorf("Expected rescaled 90000, got %d", res)
	}

	// ClockSync class
	tracks := []Track{
		{ID: 1, Timescale: 90000},
		{ID: 2, Timescale: 44100},
	}
	cs := NewClockSync(tracks)

	sec1 := cs.Normalize(1, 90000)
	if sec1 != 1.0 {
		t.Errorf("Expected 1.0s normalization, got %f", sec1)
	}

	units2 := cs.RescaleToTrack(2, 2.0)
	if units2 != 88200 {
		t.Errorf("Expected 88200 units for audio, got %d", units2)
	}

	cmp := cs.Compare(1, 45000, 2, 22050)
	if cmp != 0 {
		t.Errorf("Expected 45000 units of track 1 to equal 22050 units of track 2 (both 0.5s), got cmp=%d", cmp)
	}

	cmp = cs.Compare(1, 90000, 2, 22050)
	if cmp != 1 {
		t.Errorf("Expected 1.0s to be greater than 0.5s, got cmp=%d", cmp)
	}
}
