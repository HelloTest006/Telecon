package kaapi

import (
	"crypto/ed25519"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLStore is a SQLite-backed registry (Phase 2).
type SQLStore struct {
	db *sql.DB
	mu sync.Mutex
}

// OpenSQLStore opens or creates a SQLite database at path.
func OpenSQLStore(path string) (*SQLStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &SQLStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLStore) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS meta (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS devices (
  device_id TEXT PRIMARY KEY,
  identity_pk BLOB NOT NULL,
  sign_pk BLOB,
  org_id TEXT,
  label TEXT,
  enrollment_counter INTEGER NOT NULL,
  profile TEXT NOT NULL,
  revoked INTEGER NOT NULL DEFAULT 0,
  last_epoch_id INTEGER,
  last_serial INTEGER,
  issued_epoch INTEGER
);
CREATE TABLE IF NOT EXISTS vouchers (
  id TEXT PRIMARY KEY,
  code_hash TEXT NOT NULL UNIQUE,
  label TEXT,
  org_id TEXT,
  profile TEXT,
  max_uses INTEGER NOT NULL,
  uses INTEGER NOT NULL DEFAULT 0,
  expires_at INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  revoked INTEGER NOT NULL DEFAULT 0
);
INSERT OR IGNORE INTO meta(k,v) VALUES('serial','0');
`)
	return err
}

func (s *SQLStore) nextSerialLocked() (uint64, error) {
	var v string
	err := s.db.QueryRow(`SELECT v FROM meta WHERE k='serial'`).Scan(&v)
	if err != nil {
		return 0, err
	}
	var n uint64
	_, _ = fmt.Sscanf(v, "%d", &n)
	n++
	_, err = s.db.Exec(`UPDATE meta SET v=? WHERE k='serial'`, fmt.Sprintf("%d", n))
	return n, err
}

func (s *SQLStore) Enroll(req EnrollRequest) (*DeviceRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	existing := s.getLocked(req.DeviceID)
	if existing != nil {
		if req.EnrollmentCounter > existing.EnrollmentCounter {
			existing.EnrollmentCounter = req.EnrollmentCounter
		} else {
			existing.EnrollmentCounter++
		}
		existing.IdentityPK = append([]byte(nil), req.IdentityPK...)
		if len(req.SignPK) > 0 {
			existing.SignPK = append([]byte(nil), req.SignPK...)
		}
		existing.OrgID = req.OrgID
		existing.Label = req.Label
		existing.Profile = profile
		existing.Revoked = false
		_, err := s.db.Exec(`UPDATE devices SET identity_pk=?, sign_pk=?, org_id=?, label=?, enrollment_counter=?, profile=?, revoked=0 WHERE device_id=?`,
			existing.IdentityPK, existing.SignPK, existing.OrgID, existing.Label, existing.EnrollmentCounter, existing.Profile, existing.DeviceID)
		if err != nil {
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
	_, err := s.db.Exec(`INSERT INTO devices(device_id, identity_pk, sign_pk, org_id, label, enrollment_counter, profile, revoked)
		VALUES(?,?,?,?,?,?,?,0)`, rec.DeviceID, rec.IdentityPK, rec.SignPK, rec.OrgID, rec.Label, rec.EnrollmentCounter, rec.Profile)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *SQLStore) getLocked(deviceID string) *DeviceRecord {
	row := s.db.QueryRow(`SELECT device_id, identity_pk, sign_pk, org_id, label, enrollment_counter, profile, revoked,
		COALESCE(last_epoch_id,0), COALESCE(last_serial,0), COALESCE(issued_epoch,0) FROM devices WHERE device_id=?`, deviceID)
	var rec DeviceRecord
	var revoked int
	var signPK []byte
	err := row.Scan(&rec.DeviceID, &rec.IdentityPK, &signPK, &rec.OrgID, &rec.Label, &rec.EnrollmentCounter, &rec.Profile, &revoked,
		&rec.LastEpochID, &rec.LastSerial, &rec.IssuedEpoch)
	if err != nil {
		return nil
	}
	rec.SignPK = signPK
	rec.Revoked = revoked != 0
	return &rec
}

func (s *SQLStore) Get(deviceID string) *DeviceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLocked(deviceID)
}

func (s *SQLStore) ListDevices() []DeviceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT device_id, identity_pk, sign_pk, org_id, label, enrollment_counter, profile, revoked,
		COALESCE(last_epoch_id,0), COALESCE(last_serial,0), COALESCE(issued_epoch,0) FROM devices ORDER BY device_id`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []DeviceRecord
	for rows.Next() {
		var rec DeviceRecord
		var revoked int
		var signPK []byte
		if err := rows.Scan(&rec.DeviceID, &rec.IdentityPK, &signPK, &rec.OrgID, &rec.Label, &rec.EnrollmentCounter, &rec.Profile, &revoked,
			&rec.LastEpochID, &rec.LastSerial, &rec.IssuedEpoch); err != nil {
			continue
		}
		rec.SignPK = signPK
		rec.Revoked = revoked != 0
		out = append(out, rec)
	}
	return out
}

func (s *SQLStore) Revoke(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`UPDATE devices SET revoked=1 WHERE device_id=?`, deviceID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("not_enrolled")
	}
	return nil
}

func (s *SQLStore) MarkIssued(deviceID string, epochID uint64) (serial uint64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.getLocked(deviceID)
	if rec == nil {
		return 0, fmt.Errorf("not_enrolled")
	}
	if rec.Revoked {
		return 0, fmt.Errorf("revoked")
	}
	if rec.IssuedEpoch == epochID && rec.LastSerial != 0 {
		return rec.LastSerial, nil
	}
	serial, err = s.nextSerialLocked()
	if err != nil {
		return 0, err
	}
	_, err = s.db.Exec(`UPDATE devices SET issued_epoch=?, last_epoch_id=?, last_serial=? WHERE device_id=?`,
		epochID, epochID, serial, deviceID)
	return serial, err
}

func (s *SQLStore) RedeemVoucher(code string) (*Voucher, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := HashVoucherCode(code)
	row := s.db.QueryRow(`SELECT id, code_hash, label, org_id, profile, max_uses, uses, expires_at, created_at, revoked FROM vouchers WHERE code_hash=?`, h)
	var v Voucher
	var revoked int
	if err := row.Scan(&v.ID, &v.CodeHash, &v.Label, &v.OrgID, &v.Profile, &v.MaxUses, &v.Uses, &v.ExpiresAt, &v.CreatedAt, &revoked); err != nil {
		return nil, "invalid_voucher"
	}
	v.Revoked = revoked != 0
	if v.Revoked {
		return nil, "voucher_revoked"
	}
	if v.ExpiresAt > 0 && time.Now().Unix() > v.ExpiresAt {
		return nil, "voucher_expired"
	}
	if v.MaxUses > 0 && v.Uses >= v.MaxUses {
		return nil, "voucher_exhausted"
	}
	v.Uses++
	if _, err := s.db.Exec(`UPDATE vouchers SET uses=? WHERE id=?`, v.Uses, v.ID); err != nil {
		return nil, "internal"
	}
	return &v, ""
}

func (s *SQLStore) AddVoucher(v *Voucher) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.ID == "" {
		return fmt.Errorf("id required")
	}
	rev := 0
	if v.Revoked {
		rev = 1
	}
	_, err := s.db.Exec(`INSERT INTO vouchers(id, code_hash, label, org_id, profile, max_uses, uses, expires_at, created_at, revoked)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, v.ID, v.CodeHash, v.Label, v.OrgID, v.Profile, v.MaxUses, v.Uses, v.ExpiresAt, v.CreatedAt, rev)
	return err
}

func (s *SQLStore) ListVouchers() []VoucherInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT id, label, org_id, profile, max_uses, uses, expires_at, created_at, revoked FROM vouchers ORDER BY created_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []VoucherInfo
	for rows.Next() {
		var vi VoucherInfo
		var rev int
		if err := rows.Scan(&vi.ID, &vi.Label, &vi.OrgID, &vi.Profile, &vi.MaxUses, &vi.Uses, &vi.ExpiresAt, &vi.CreatedAt, &rev); err != nil {
			continue
		}
		vi.Revoked = rev != 0
		out = append(out, vi)
	}
	return out
}

func (s *SQLStore) RevokeVoucher(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`UPDATE vouchers SET revoked=1 WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("not_found")
	}
	return nil
}

// Close closes the database.
func (s *SQLStore) Close() error {
	return s.db.Close()
}
