package device

import (
	"fmt"
	"time"

	coecrypto "github.com/telecon/coe/internal/crypto"
)

// Now is overridable for tests.
var Now = time.Now

// CurrentEpoch returns epoch for wall clock.
func CurrentEpoch() uint64 {
	return coecrypto.EpochID(Now())
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
