package coecrypto

import "fmt"

// DeriveGeneralKey computes the per-device daily general key.
func DeriveGeneralKey(kaMaster []byte, epochID uint64, deviceID string, enrollmentCounter uint32) ([]byte, error) {
	if len(kaMaster) != 32 {
		return nil, fmt.Errorf("KA_MASTER must be 32 bytes")
	}
	salt := Concat(I2OSP(epochID, 8), []byte("COE-v1-salt"))
	info := Concat([]byte("COE-v1-general"), []byte(deviceID), I2OSP(uint64(enrollmentCounter), 4))
	return HKDF(kaMaster, salt, info, 32)
}

// DeriveOrgDayKey computes the org-wide daily key (COE-Simple).
func DeriveOrgDayKey(kaMaster []byte, epochID uint64, orgID string) ([]byte, error) {
	if len(kaMaster) != 32 {
		return nil, fmt.Errorf("KA_MASTER must be 32 bytes")
	}
	salt := Concat(I2OSP(epochID, 8), []byte("COE-v1-salt"))
	info := Concat([]byte("COE-v1-org-day"), []byte(orgID))
	return HKDF(kaMaster, salt, info, 32)
}
