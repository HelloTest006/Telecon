package kaapi

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// DeviceRecord is enrolled device state.
type DeviceRecord struct {
	DeviceID          string `json:"device_id"`
	IdentityPK        []byte `json:"identity_pk"`
	SignPK            []byte `json:"sign_pk,omitempty"`
	OrgID             string `json:"org_id,omitempty"`
	Label             string `json:"label,omitempty"`
	EnrollmentCounter uint32 `json:"enrollment_counter"`
	Profile           string `json:"profile"`
	Revoked           bool   `json:"revoked"`
	LastEpochID       uint64 `json:"last_epoch_id,omitempty"`
	LastSerial        uint64 `json:"last_serial,omitempty"`
	IssuedEpoch       uint64 `json:"issued_epoch,omitempty"`
}

// Registry is a JSON-file backed device registry.
type Registry struct {
	path     string
	mu       sync.Mutex
	Devices  map[string]*DeviceRecord `json:"devices"`
	Vouchers map[string]*Voucher      `json:"vouchers,omitempty"`
	Serial   uint64                   `json:"serial"`
}

func LoadRegistry(path string) (*Registry, error) {
	r := &Registry{
		path:     path,
		Devices:  make(map[string]*DeviceRecord),
		Vouchers: make(map[string]*Voucher),
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, r.Save()
		}
		return nil, err
	}
	if err := json.Unmarshal(b, r); err != nil {
		return nil, err
	}
	if r.Devices == nil {
		r.Devices = make(map[string]*DeviceRecord)
	}
	if r.Vouchers == nil {
		r.Vouchers = make(map[string]*Voucher)
	}
	r.path = path
	return r, nil
}

func (r *Registry) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saveLocked()
}

func (r *Registry) saveLocked() error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, b, 0o600)
}

func (r *Registry) Enroll(req EnrollRequest) (*DeviceRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if req.DeviceID == "" {
		return nil, fmt.Errorf("device_id required")
	}
	if len(req.IdentityPK) != 32 {
		return nil, fmt.Errorf("identity_pk must be 32 bytes")
	}
	if len(req.SignPK) != 0 && len(req.SignPK) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("sign_pk must be %d bytes", ed25519.PublicKeySize)
	}
	profile := req.Profile
	if profile == "" {
		profile = "strong"
	}
	counter := req.EnrollmentCounter
	if counter == 0 {
		counter = 1
	}
	if existing, ok := r.Devices[req.DeviceID]; ok {
		if req.EnrollmentCounter > existing.EnrollmentCounter {
			existing.EnrollmentCounter = req.EnrollmentCounter
		} else {
			existing.EnrollmentCounter++
			counter = existing.EnrollmentCounter
		}
		existing.IdentityPK = append([]byte(nil), req.IdentityPK...)
		if len(req.SignPK) > 0 {
			existing.SignPK = append([]byte(nil), req.SignPK...)
		}
		existing.OrgID = req.OrgID
		existing.Label = req.Label
		existing.Profile = profile
		existing.Revoked = false
		if err := r.saveLocked(); err != nil {
			return nil, err
		}
		return existing, nil
	}
	rec := &DeviceRecord{
		DeviceID:          req.DeviceID,
		IdentityPK:        append([]byte(nil), req.IdentityPK...),
		SignPK:            append([]byte(nil), req.SignPK...),
		OrgID:             req.OrgID,
		Label:             req.Label,
		EnrollmentCounter: counter,
		Profile:           profile,
	}
	r.Devices[req.DeviceID] = rec
	if err := r.saveLocked(); err != nil {
		return nil, err
	}
	return rec, nil
}

func (r *Registry) Get(deviceID string) *DeviceRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Devices[deviceID]
}

func (r *Registry) Revoke(deviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.Devices[deviceID]
	if !ok {
		return fmt.Errorf("not_enrolled")
	}
	rec.Revoked = true
	return r.saveLocked()
}

func (r *Registry) MarkIssued(deviceID string, epochID uint64) (serial uint64, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.Devices[deviceID]
	if !ok {
		return 0, fmt.Errorf("not_enrolled")
	}
	if rec.Revoked {
		return 0, fmt.Errorf("revoked")
	}
	if rec.IssuedEpoch == epochID && rec.LastSerial != 0 {
		return rec.LastSerial, nil
	}
	r.Serial++
	serial = r.Serial
	rec.IssuedEpoch = epochID
	rec.LastEpochID = epochID
	rec.LastSerial = serial
	if err := r.saveLocked(); err != nil {
		return 0, err
	}
	return serial, nil
}
