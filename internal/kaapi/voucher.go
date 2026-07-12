package kaapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Voucher is a one-time (or limited) enroll grant. Raw code never stored — only hash.
type Voucher struct {
	ID        string `json:"id"`
	CodeHash  string `json:"code_hash"` // hex(sha256(code))
	Label     string `json:"label,omitempty"`
	OrgID     string `json:"org_id,omitempty"`
	Profile   string `json:"profile,omitempty"`
	MaxUses   int    `json:"max_uses"`
	Uses      int    `json:"uses"`
	ExpiresAt int64  `json:"expires_at"` // unix; 0 = never
	CreatedAt int64  `json:"created_at"`
	Revoked   bool   `json:"revoked"`
}

// HashVoucherCode returns hex SHA-256 of normalized code.
func HashVoucherCode(code string) string {
	code = strings.TrimSpace(code)
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// GenerateVoucherCode returns a human-shareable code (32 hex chars).
func GenerateVoucherCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateVoucherRequest is admin POST /v1/vouchers.
type CreateVoucherRequest struct {
	Label     string `json:"label,omitempty"`
	OrgID     string `json:"org_id,omitempty"`
	Profile   string `json:"profile,omitempty"`
	MaxUses   int    `json:"max_uses"`             // default 1
	TTLHours  int    `json:"ttl_hours,omitempty"`  // default 168 (7d); 0 = no expiry if MaxUses set
	Code      string `json:"code,omitempty"`       // optional custom code
}

// CreateVoucherResponse returns the plaintext code once.
type CreateVoucherResponse struct {
	ID        string `json:"id"`
	Code      string `json:"code"` // shown once
	MaxUses   int    `json:"max_uses"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
	Label     string `json:"label,omitempty"`
	Profile   string `json:"profile,omitempty"`
	OrgID     string `json:"org_id,omitempty"`
}

// VoucherInfo is public listing without code.
type VoucherInfo struct {
	ID        string `json:"id"`
	Label     string `json:"label,omitempty"`
	OrgID     string `json:"org_id,omitempty"`
	Profile   string `json:"profile,omitempty"`
	MaxUses   int    `json:"max_uses"`
	Uses      int    `json:"uses"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
	Revoked   bool   `json:"revoked"`
	CreatedAt int64  `json:"created_at"`
}

// Redeem applies a voucher; returns error code string or "".
func (r *Registry) RedeemVoucher(code string) (*Voucher, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Vouchers == nil {
		return nil, "invalid_voucher"
	}
	h := HashVoucherCode(code)
	var found *Voucher
	for _, v := range r.Vouchers {
		if v.CodeHash == h {
			found = v
			break
		}
	}
	if found == nil {
		return nil, "invalid_voucher"
	}
	if found.Revoked {
		return nil, "voucher_revoked"
	}
	if found.ExpiresAt > 0 && time.Now().Unix() > found.ExpiresAt {
		return nil, "voucher_expired"
	}
	if found.MaxUses > 0 && found.Uses >= found.MaxUses {
		return nil, "voucher_exhausted"
	}
	found.Uses++
	if err := r.saveLocked(); err != nil {
		found.Uses--
		return nil, "internal"
	}
	return found, ""
}

// AddVoucher stores a new voucher (code already hashed).
func (r *Registry) AddVoucher(v *Voucher) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Vouchers == nil {
		r.Vouchers = make(map[string]*Voucher)
	}
	if v.ID == "" {
		return fmt.Errorf("id required")
	}
	r.Vouchers[v.ID] = v
	return r.saveLocked()
}

// ListVouchers returns non-secret voucher info.
func (r *Registry) ListVouchers() []VoucherInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]VoucherInfo, 0, len(r.Vouchers))
	for _, v := range r.Vouchers {
		out = append(out, VoucherInfo{
			ID: v.ID, Label: v.Label, OrgID: v.OrgID, Profile: v.Profile,
			MaxUses: v.MaxUses, Uses: v.Uses, ExpiresAt: v.ExpiresAt,
			Revoked: v.Revoked, CreatedAt: v.CreatedAt,
		})
	}
	return out
}

// RevokeVoucher marks voucher revoked.
func (r *Registry) RevokeVoucher(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.Vouchers[id]
	if !ok {
		return fmt.Errorf("not_found")
	}
	v.Revoked = true
	return r.saveLocked()
}
