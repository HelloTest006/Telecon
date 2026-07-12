package kaapi

// IssueRequest is POST /v1/key/issue body.
type IssueRequest struct {
	DeviceID   string `json:"device_id"`
	EpochHint  uint64 `json:"epoch_hint,omitempty"`
	Nonce      []byte `json:"nonce"`
	ClientTime int64  `json:"client_time"`
	Profile    string `json:"profile,omitempty"`
	// Signature is Ed25519 over IssueSignPayload (required for production auth).
	Signature []byte `json:"signature,omitempty"`
}

// IssueResponse is successful key issue.
type IssueResponse struct {
	DeviceID     string `json:"device_id"`
	EpochID      uint64 `json:"epoch_id"`
	NotBefore    int64  `json:"not_before"`
	NotAfter     int64  `json:"not_after"`
	GraceSeconds uint64 `json:"grace_seconds"`
	KeySerial    uint64 `json:"key_serial"`
	RequestID    []byte `json:"request_id"`
	PublicTag    []byte `json:"public_tag,omitempty"`
	GeneralKey   []byte `json:"general_key"`
	Profile      string `json:"profile"`
	KATime       int64  `json:"ka_time"`
}

// EnrollRequest is POST /v1/enroll.
// Auth: admin Bearer OR one-time voucher_code (no admin token on device).
type EnrollRequest struct {
	DeviceID          string `json:"device_id"`
	IdentityPK        []byte `json:"identity_pk"` // X25519
	SignPK            []byte `json:"sign_pk"`     // Ed25519 public
	OrgID             string `json:"org_id,omitempty"`
	Label             string `json:"label,omitempty"`
	EnrollmentCounter uint32 `json:"enrollment_counter,omitempty"`
	Profile           string `json:"profile,omitempty"`
	VoucherCode       string `json:"voucher_code,omitempty"`
}

// EnrollResponse is 201 body.
type EnrollResponse struct {
	DeviceID          string `json:"device_id"`
	EnrollmentCounter uint32 `json:"enrollment_counter"`
	Profile           string `json:"profile"`
}

// StatusResponse is GET /v1/key/status.
type StatusResponse struct {
	DeviceID        string `json:"device_id"`
	Revoked         bool   `json:"revoked"`
	CurrentEpochID  uint64 `json:"current_epoch_id"`
	IssuedThisEpoch bool   `json:"issued_this_epoch"`
	KeySerial       uint64 `json:"key_serial,omitempty"`
	NotAfter        int64  `json:"not_after,omitempty"`
}

// RevokeRequest is POST /v1/revoke.
type RevokeRequest struct {
	DeviceID string `json:"device_id"`
	Reason   string `json:"reason,omitempty"`
}

// HealthResponse is GET /v1/health.
type HealthResponse struct {
	Status  string `json:"status"`
	Time    int64  `json:"time"`
	EpochID uint64 `json:"epoch_id"`
	TLS     bool   `json:"tls"`
}

// ErrorBody is JSON error.
type ErrorBody struct {
	Error string `json:"error"`
}
