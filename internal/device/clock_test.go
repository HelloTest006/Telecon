package device

import (
	"testing"
	"time"
)

func TestApplyKATime(t *testing.T) {
	SetClockOffset(0)
	// pretend local is 100s behind KA
	base := time.Unix(1_700_000_000, 0).UTC()
	Now = func() time.Time { return base }
	defer func() { Now = time.Now }()

	ApplyKATime(base.Unix() + 30)
	off := ClockOffset()
	if off < 29*time.Second || off > 31*time.Second {
		t.Fatalf("offset=%v want ~30s", off)
	}
	ct := CorrectedNow().Unix()
	if ct != base.Unix()+30 {
		t.Fatalf("corrected=%d", ct)
	}
}
