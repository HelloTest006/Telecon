package coecrypto

import (
	"fmt"
)

// SessionParams holds inputs for SessionRoot derivation (COE-Strong).
type SessionParams struct {
	EpochID        uint64
	DeviceIDA      string
	DeviceIDB      string
	GeneralKeyA    []byte
	GeneralKeyB    []byte
	IdentitySk     []byte
	IdentityPkPeer []byte
	EphSk          []byte
	EphPkPeer      []byte
	NonceA         []byte
	NonceB         []byte
	Profile        string
	OrgDayKey      []byte
	LocalDeviceID  string
	PeerDeviceID   string
}

func orderedIDs(a, b string) (lo, hi string) {
	if a < b {
		return a, b
	}
	return b, a
}

// DeriveWrapKey builds a temporary key to seal GeneralKeys during handshake.
// Does not use daily keys — only ECDH + nonces + epoch (so peers can exchange GeneralKeys privately).
func DeriveWrapKey(
	identitySk, identityPkPeer, ephSk, ephPkPeer []byte,
	nonceA, nonceB []byte,
	epochID uint64,
	deviceIDA, deviceIDB string,
) ([]byte, error) {
	ssStatic, err := X25519(identitySk, identityPkPeer)
	if err != nil {
		return nil, err
	}
	var ssEph []byte
	if len(ephSk) == 32 && len(ephPkPeer) == 32 {
		ssEph, err = X25519(ephSk, ephPkPeer)
		if err != nil {
			return nil, err
		}
	}
	var ikm []byte
	if ssEph != nil {
		ikm = Concat(ssStatic, ssEph)
	} else {
		ikm = ssStatic
	}
	idLo, idHi := orderedIDs(deviceIDA, deviceIDB)
	var salt []byte
	if deviceIDA < deviceIDB {
		salt = Concat(nonceA, nonceB)
	} else {
		salt = Concat(nonceB, nonceA)
	}
	info := Concat([]byte("COE-v1-gk-wrap"), I2OSP(epochID, 8), []byte(idLo), []byte(idHi))
	return HKDF(ikm, salt, info, 32)
}

// DeriveSessionStrong derives SessionRoot and directional send keys.
func DeriveSessionStrong(p SessionParams) (sessionRoot, localSend, peerSend []byte, err error) {
	if p.Profile == "" {
		p.Profile = "strong"
	}
	idLo, idHi := orderedIDs(p.DeviceIDA, p.DeviceIDB)

	var ikm []byte
	if p.Profile == "simple" {
		if len(p.OrgDayKey) != 32 {
			return nil, nil, nil, fmt.Errorf("simple profile needs OrgDayKey")
		}
		ikm = p.OrgDayKey
	} else {
		if len(p.IdentitySk) != 32 || len(p.IdentityPkPeer) != 32 {
			return nil, nil, nil, fmt.Errorf("strong profile needs identity keys")
		}
		if len(p.GeneralKeyA) != 32 || len(p.GeneralKeyB) != 32 {
			return nil, nil, nil, fmt.Errorf("strong profile needs both general keys")
		}
		ssStatic, err := X25519(p.IdentitySk, p.IdentityPkPeer)
		if err != nil {
			return nil, nil, nil, err
		}
		var ssEph []byte
		if len(p.EphSk) == 32 && len(p.EphPkPeer) == 32 {
			ssEph, err = X25519(p.EphSk, p.EphPkPeer)
			if err != nil {
				return nil, nil, nil, err
			}
		}
		var gk []byte
		if p.DeviceIDA < p.DeviceIDB {
			gk = Concat(p.GeneralKeyA, p.GeneralKeyB)
		} else {
			gk = Concat(p.GeneralKeyB, p.GeneralKeyA)
		}
		if ssEph != nil {
			ikm = Concat(ssStatic, ssEph, gk)
		} else {
			ikm = Concat(ssStatic, gk)
		}
	}

	var salt []byte
	if p.DeviceIDA < p.DeviceIDB {
		salt = Concat(p.NonceA, p.NonceB)
	} else {
		salt = Concat(p.NonceB, p.NonceA)
	}

	infoPrefix := "COE-v1-session"
	if p.Profile == "simple" {
		infoPrefix = "COE-v1-session-simple"
	}
	info := Concat([]byte(infoPrefix), I2OSP(p.EpochID, 8), []byte(idLo), []byte(idHi), []byte(p.Profile))

	sessionRoot, err = HKDF(ikm, salt, info, 32)
	if err != nil {
		return nil, nil, nil, err
	}

	kA, err := HKDF(sessionRoot, ZeroSalt32, Concat([]byte("COE-v1-send"), []byte(p.DeviceIDA)), 32)
	if err != nil {
		return nil, nil, nil, err
	}
	kB, err := HKDF(sessionRoot, ZeroSalt32, Concat([]byte("COE-v1-send"), []byte(p.DeviceIDB)), 32)
	if err != nil {
		return nil, nil, nil, err
	}

	if p.LocalDeviceID == p.DeviceIDA {
		return sessionRoot, kA, kB, nil
	}
	if p.LocalDeviceID == p.DeviceIDB {
		return sessionRoot, kB, kA, nil
	}
	return nil, nil, nil, fmt.Errorf("LocalDeviceID must be DeviceIDA or DeviceIDB")
}
