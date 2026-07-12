package p2p

// ReplayWindow is a sliding window for seq numbers.
type ReplayWindow struct {
	size   uint64
	maxSeq uint64
	bitmap map[uint64]struct{}
	inited bool
}

// NewReplayWindow creates window of given size (e.g. 128).
func NewReplayWindow(size uint64) *ReplayWindow {
	if size == 0 {
		size = 128
	}
	return &ReplayWindow{size: size, bitmap: make(map[uint64]struct{})}
}

// Accept returns true if seq is new and within window.
func (w *ReplayWindow) Accept(seq uint64) bool {
	if !w.inited {
		w.inited = true
		w.maxSeq = seq
		w.bitmap[seq] = struct{}{}
		return true
	}
	if _, seen := w.bitmap[seq]; seen {
		return false
	}
	if seq+w.size < w.maxSeq {
		// too old
		return false
	}
	if seq > w.maxSeq {
		// advance: drop below maxSeq-size+1
		low := seq - w.size + 1
		if seq < w.size {
			low = 0
		}
		for s := range w.bitmap {
			if s < low {
				delete(w.bitmap, s)
			}
		}
		w.maxSeq = seq
	}
	w.bitmap[seq] = struct{}{}
	return true
}
