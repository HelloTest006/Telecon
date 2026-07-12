package coecrypto

import "time"

const (
	// EpochSeconds is 24h UTC day boundary unit.
	EpochSeconds int64 = 86400
	// DefaultGraceSeconds after epoch end for in-flight sessions.
	DefaultGraceSeconds int64 = 300
	// MaxClockSkewSeconds for KA issue.
	MaxClockSkewSeconds int64 = 120
)

// EpochID returns floor(unix/86400).
func EpochID(t time.Time) uint64 {
	return uint64(t.UTC().Unix() / EpochSeconds)
}

// EpochBounds returns not_before, not_after for epoch.
func EpochBounds(epochID uint64) (notBefore, notAfter int64) {
	notBefore = int64(epochID) * EpochSeconds
	notAfter = notBefore + EpochSeconds
	return
}

// EpochValidForUse reports whether epochID may still be used at now (current or prev within grace).
func EpochValidForUse(epochID uint64, now time.Time, grace int64) bool {
	if grace <= 0 {
		grace = DefaultGraceSeconds
	}
	cur := EpochID(now)
	if epochID == cur {
		return true
	}
	if epochID+1 == cur {
		_, end := EpochBounds(epochID)
		return now.UTC().Unix() < end+grace
	}
	return false
}

// AgreeEpoch picks agreed epoch for handshake.
func AgreeEpoch(epochA, epochB uint64, now time.Time, grace int64) (uint64, bool) {
	if grace <= 0 {
		grace = DefaultGraceSeconds
	}
	if epochA == epochB {
		if EpochValidForUse(epochA, now, grace) {
			return epochA, true
		}
		return 0, false
	}
	// Prefer previous epoch if one side still on prev within grace and other on current.
	var prev, newer uint64
	if epochA < epochB {
		prev, newer = epochA, epochB
	} else {
		prev, newer = epochB, epochA
	}
	cur := EpochID(now)
	if newer == cur && prev+1 == newer && EpochValidForUse(prev, now, grace) {
		return prev, true
	}
	return 0, false
}
