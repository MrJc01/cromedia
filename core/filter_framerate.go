package core

// CFRFilter converts a sequence of packets to a Constant Frame Rate (CFR).
type CFRFilter struct {
	TargetFPS     float64
	Timescale     uint32
	frameDuration int64
	nextPTS       int64
	lastPacket    *Packet
	initialized   bool
}

// NewCFRFilter initializes a CFRFilter with a target FPS and timescale.
func NewCFRFilter(targetFPS float64, timescale uint32) *CFRFilter {
	frameDuration := int64(float64(timescale) / targetFPS)
	if frameDuration <= 0 {
		frameDuration = 1
	}
	return &CFRFilter{
		TargetFPS:     targetFPS,
		Timescale:     timescale,
		frameDuration: frameDuration,
	}
}

// Process processes a single input packet and returns zero or more output packets.
// If input is finished, passing nil will not yield more since CFR is fully reactive.
func (f *CFRFilter) Process(pkt *Packet) []*Packet {
	if pkt == nil {
		return nil
	}

	if !f.initialized {
		f.nextPTS = pkt.PTS
		f.initialized = true
	}

	var out []*Packet

	// If the current packet PTS is in the future compared to nextPTS,
	// duplicate the last packet to fill the gap.
	for pkt.PTS >= f.nextPTS+f.frameDuration/2 {
		if f.lastPacket != nil {
			dup := &Packet{
				ID:          NewPacketID(),
				StreamIndex: f.lastPacket.StreamIndex,
				Data:        f.lastPacket.Data,
				PTS:         f.nextPTS,
				DTS:         f.nextPTS,
				Duration:    f.frameDuration,
				IsKeyframe:  false,
			}
			out = append(out, dup)
			f.nextPTS += f.frameDuration
		} else {
			f.nextPTS = pkt.PTS
			break
		}
	}

	// If the incoming packet is within or ahead of the next expected PTS window, emit it.
	if pkt.PTS >= f.nextPTS-f.frameDuration/2 {
		emitPkt := &Packet{
			ID:          NewPacketID(),
			StreamIndex: pkt.StreamIndex,
			Data:        pkt.Data,
			PTS:         f.nextPTS,
			DTS:         f.nextPTS,
			Duration:    f.frameDuration,
			IsKeyframe:  pkt.IsKeyframe,
		}
		out = append(out, emitPkt)
		f.nextPTS += f.frameDuration
	}

	f.lastPacket = pkt
	return out
}

// VFRFilter normalizes and cleans up timestamps for Variable Frame Rate streams.
type VFRFilter struct {
	lastPTS     int64
	initialized bool
}

// NewVFRFilter initializes a VFRFilter.
func NewVFRFilter() *VFRFilter {
	return &VFRFilter{}
}

// Process processes a packet, ensuring strictly increasing timestamps and valid durations.
func (f *VFRFilter) Process(pkt *Packet) *Packet {
	if pkt == nil {
		return nil
	}

	if !f.initialized {
		f.lastPTS = pkt.PTS
		f.initialized = true
		return pkt
	}

	// Ensure strictly monotonically increasing PTS
	if pkt.PTS <= f.lastPTS {
		pkt.PTS = f.lastPTS + 1
	}
	if pkt.DTS > pkt.PTS {
		pkt.DTS = pkt.PTS
	} else if pkt.DTS <= f.lastPTS {
		pkt.DTS = f.lastPTS + 1
	}

	pkt.Duration = pkt.PTS - f.lastPTS
	f.lastPTS = pkt.PTS
	return pkt
}
