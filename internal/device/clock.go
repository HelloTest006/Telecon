package device

import (
	"fmt"
	"sync"
	"time"

	coecrypto "github.com/telecon/coe/internal/crypto"
)

// Now is overridable for tests. Prefer CorrectedNow for issue/enroll timestamps.
var Now = time.Now

var (
	offsetMu   sync.RWMutex
	clockOffset time.Duration // add to wall clock to approximate KA time
)

// SetClockOffset stores server-client offset (serverUnix - localUnix as duration).
func SetClockOffset(d time.Duration) {
	offsetMu.Lock()
	clockOffset = d
	offsetMu.Unlock()
}

// ClockOffset returns current correction duration.
func ClockOffset() time.Duration {
	offsetMu.RLock()
	defer offsetMu.RUnlock()
	return clockOffset
}

// CorrectedNow returns wall clock adjusted toward last KA time.
func CorrectedNow() time.Time {
	return Now().Add(ClockOffset())
}

// ApplyKATime updates offset from KA response ka_time (unix seconds).
// Caps extreme corrections so a bad server cannot jump years.
func ApplyKATime(kaUnix int64) {
	if kaUnix <= 0 {
		return
	}
	local := Now().UTC().Unix()
	delta := time.Duration(kaUnix-local) * time.Second
	// ignore tiny noise under 1s
	if delta < time.Second && delta > -time.Second {
		return
	}
	// cap at 2 hours absolute for safety
	max := 2 * time.Hour
	if delta > max {
		delta = max
	}
	if delta < -max {
		delta = -max
	}
	SetClockOffset(delta)
}

// CurrentEpoch returns epoch for corrected wall clock.
func CurrentEpoch() uint64 {
	return coecrypto.EpochID(CorrectedNow())
}

// CheckSkew returns error if |client - server| > max skew.
func CheckSkew(clientUnix, serverUnix int64) error {
	d := clientUnix - serverUnix
	if d < 0 {
		d = -d
	}
	if d > coecrypto.MaxClockSkewSeconds {
		return fmt.Errorf("clock_skew: delta=%d", d)
	}
	return nil
}
