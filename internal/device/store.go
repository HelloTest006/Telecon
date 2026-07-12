package device

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Meta is non-secret metadata for a stored general key.
type Meta struct {
	EpochID   uint64 `json:"epoch_id"`
	KeySerial uint64 `json:"key_serial"`
	NotBefore int64  `json:"not_before"`
	NotAfter  int64  `json:"not_after"`
	GraceSec  int64  `json:"grace_seconds"`
	Profile   string `json:"profile"`
	DeviceID  string `json:"device_id"`
}

type record struct {
	// EncKey is DPAPI/protected blob of the 32-byte general key.
	EncKey []byte `json:"enc_key"`
	// Legacy plaintext (migrated away).
	Key  []byte `json:"key,omitempty"`
	Meta Meta   `json:"meta"`
}

// FileStore persists general keys under a directory (DPAPI-wrapped on Windows).
type FileStore struct {
	dir string
	mu  sync.Mutex
}

func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &FileStore{dir: dir}, nil
}

func (s *FileStore) path(epoch uint64) string {
	return filepath.Join(s.dir, fmt.Sprintf("general-%d.json", epoch))
}

func (s *FileStore) PutGeneral(epoch uint64, key []byte, meta Meta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta.EpochID = epoch
	enc, err := Protect(key)
	if err != nil {
		return err
	}
	rec := record{EncKey: enc, Meta: meta}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(epoch), b, 0o600)
}

func (s *FileStore) GetGeneral(epoch uint64) ([]byte, Meta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path(epoch))
	if err != nil {
		return nil, Meta{}, err
	}
	var rec record
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, Meta{}, err
	}
	var key []byte
	if len(rec.EncKey) > 0 {
		key, err = Unprotect(rec.EncKey)
		if err != nil {
			return nil, Meta{}, err
		}
	} else if len(rec.Key) > 0 {
		key = append([]byte(nil), rec.Key...)
	} else {
		return nil, Meta{}, fmt.Errorf("empty key record")
	}
	return key, rec.Meta, nil
}

func (s *FileStore) WipeBefore(before uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		var ep uint64
		if _, err := fmt.Sscanf(e.Name(), "general-%d.json", &ep); err != nil {
			continue
		}
		if ep < before {
			_ = os.Remove(filepath.Join(s.dir, e.Name()))
		}
	}
	return nil
}

// Identity is long-term device material.
// Private keys stored DPAPI-protected when written via SaveIdentity.
type Identity struct {
	DeviceID          string `json:"device_id"`
	EnrollmentCounter uint32 `json:"enrollment_counter"`
	// ECDH (X25519)
	PrivateKey []byte `json:"-"` // loaded in memory only
	PublicKey  []byte `json:"public_key"`
	// Ed25519 for KA request signing
	SignPrivate []byte `json:"-"`
	SignPublic  []byte `json:"sign_public"`
	// On-disk protected blobs
	EncPrivateKey []byte `json:"enc_private_key,omitempty"`
	EncSignKey    []byte `json:"enc_sign_key,omitempty"`
	// Legacy plaintext fields (migrated on load)
	LegacyPrivateKey []byte `json:"private_key,omitempty"`
	LegacySignKey    []byte `json:"sign_private,omitempty"`
	OrgID            string `json:"org_id,omitempty"`
	Profile          string `json:"profile"`
}

// diskIdentity is JSON shape.
type diskIdentity struct {
	DeviceID          string `json:"device_id"`
	EnrollmentCounter uint32 `json:"enrollment_counter"`
	PublicKey         []byte `json:"public_key"`
	SignPublic        []byte `json:"sign_public"`
	EncPrivateKey     []byte `json:"enc_private_key,omitempty"`
	EncSignKey        []byte `json:"enc_sign_key,omitempty"`
	PrivateKey        []byte `json:"private_key,omitempty"`
	SignPrivate       []byte `json:"sign_private,omitempty"`
	OrgID             string `json:"org_id,omitempty"`
	Profile           string `json:"profile"`
}

// SaveIdentity writes identity with protected private keys.
func SaveIdentity(path string, id Identity) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	d := diskIdentity{
		DeviceID:          id.DeviceID,
		EnrollmentCounter: id.EnrollmentCounter,
		PublicKey:         id.PublicKey,
		SignPublic:        id.SignPublic,
		OrgID:             id.OrgID,
		Profile:           id.Profile,
	}
	var err error
	if len(id.PrivateKey) > 0 {
		d.EncPrivateKey, err = Protect(id.PrivateKey)
		if err != nil {
			return err
		}
	}
	if len(id.SignPrivate) > 0 {
		d.EncSignKey, err = Protect(id.SignPrivate)
		if err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// LoadIdentity reads and unprotects private keys.
func LoadIdentity(path string) (Identity, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Identity{}, err
	}
	var d diskIdentity
	if err := json.Unmarshal(b, &d); err != nil {
		return Identity{}, err
	}
	id := Identity{
		DeviceID:          d.DeviceID,
		EnrollmentCounter: d.EnrollmentCounter,
		PublicKey:         d.PublicKey,
		SignPublic:        d.SignPublic,
		OrgID:             d.OrgID,
		Profile:           d.Profile,
	}
	if len(d.EncPrivateKey) > 0 {
		id.PrivateKey, err = Unprotect(d.EncPrivateKey)
		if err != nil {
			return Identity{}, fmt.Errorf("unprotect ecdh: %w", err)
		}
	} else if len(d.PrivateKey) > 0 {
		id.PrivateKey = d.PrivateKey
	}
	if len(d.EncSignKey) > 0 {
		id.SignPrivate, err = Unprotect(d.EncSignKey)
		if err != nil {
			return Identity{}, fmt.Errorf("unprotect sign: %w", err)
		}
	} else if len(d.SignPrivate) > 0 {
		id.SignPrivate = d.SignPrivate
	}
	// Migrate legacy plaintext → protected on next save opportunity
	if (len(d.PrivateKey) > 0 || len(d.SignPrivate) > 0) && (len(d.EncPrivateKey) == 0 || len(d.EncSignKey) == 0) {
		_ = SaveIdentity(path, id)
	}
	return id, nil
}

// SignPrivateKey returns ed25519 private for signing.
func (id Identity) SignPrivateKey() (ed25519.PrivateKey, error) {
	return SignSKFromBytes(id.SignPrivate)
}
