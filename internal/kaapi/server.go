package kaapi

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	coecrypto "github.com/telecon/coe/internal/crypto"
	"github.com/telecon/coe/internal/device"
)

// Server is the Key Authority HTTP API.
type Server struct {
	Master   []byte
	Registry *Registry
	XO       *coecrypto.Xoroshiro128pp
	Audit    *Auditor
	AdminTok string
	// RequireSignature forces Ed25519 on /v1/key/issue (default true).
	RequireSignature bool
	// RequireAdminToken rejects empty admin token for admin routes.
	RequireAdminToken bool
	// TLSActive for health reporting.
	TLSActive bool
	// Limit optional rate limiter (wrapped by HandlerWithLimit).
	Limit *RateLimiter
}

func (s *Server) writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) writeErr(w http.ResponseWriter, code int, errCode string) {
	s.writeJSON(w, code, ErrorBody{Error: errCode})
}

func (s *Server) adminOK(r *http.Request) bool {
	if s.AdminTok == "" {
		return !s.RequireAdminToken
	}
	return r.Header.Get("Authorization") == "Bearer "+s.AdminTok
}

// Handler returns the HTTP mux (no rate limit).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("POST /v1/enroll", s.handleEnroll)
	mux.HandleFunc("POST /v1/key/issue", s.handleIssue)
	mux.HandleFunc("GET /v1/key/status", s.handleStatus)
	mux.HandleFunc("POST /v1/revoke", s.handleRevoke)
	mux.HandleFunc("POST /v1/vouchers", s.handleCreateVoucher)
	mux.HandleFunc("GET /v1/vouchers", s.handleListVouchers)
	mux.HandleFunc("POST /v1/vouchers/revoke", s.handleRevokeVoucher)
	return mux
}

// HandlerWithLimit wraps Handler with rate limiting when Limit set.
func (s *Server) HandlerWithLimit() http.Handler {
	h := s.Handler()
	if s.Limit != nil {
		return s.Limit.Middleware(h)
	}
	return h
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	now := device.Now().UTC()
	s.writeJSON(w, 200, HealthResponse{
		Status:  "ok",
		Time:    now.Unix(),
		EpochID: coecrypto.EpochID(now),
		TLS:     s.TLSActive || r.TLS != nil,
	})
}

func (s *Server) handleEnroll(w http.ResponseWriter, r *http.Request) {
	var req EnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErr(w, 400, "bad_request")
		return
	}

	admin := s.adminOK(r)
	var voucher *Voucher
	if !admin {
		if req.VoucherCode == "" {
			s.writeErr(w, 401, "unauthorized")
			s.Audit.Log(AuditEvent{Endpoint: "/v1/enroll", ResultCode: 401, SrcIP: r.RemoteAddr, Error: "unauthorized"})
			return
		}
		v, errCode := s.Registry.RedeemVoucher(req.VoucherCode)
		if errCode != "" {
			s.writeErr(w, 403, errCode)
			s.Audit.Log(AuditEvent{Endpoint: "/v1/enroll", DeviceID: req.DeviceID, ResultCode: 403, Error: errCode, SrcIP: r.RemoteAddr})
			return
		}
		voucher = v
		// voucher may set org/profile defaults
		if req.OrgID == "" && v.OrgID != "" {
			req.OrgID = v.OrgID
		}
		if req.Profile == "" && v.Profile != "" {
			req.Profile = v.Profile
		}
		if req.Label == "" && v.Label != "" {
			req.Label = v.Label
		}
	}

	rec, err := s.Registry.Enroll(req)
	if err != nil {
		s.writeErr(w, 400, "bad_request")
		s.Audit.Log(AuditEvent{Endpoint: "/v1/enroll", DeviceID: req.DeviceID, ResultCode: 400, Error: err.Error(), SrcIP: r.RemoteAddr})
		return
	}
	via := "admin"
	if voucher != nil {
		via = "voucher:" + voucher.ID
	}
	s.Audit.Log(AuditEvent{Endpoint: "/v1/enroll", DeviceID: rec.DeviceID, ResultCode: 201, Profile: rec.Profile, SrcIP: r.RemoteAddr, Error: via})
	s.writeJSON(w, 201, EnrollResponse{
		DeviceID:          rec.DeviceID,
		EnrollmentCounter: rec.EnrollmentCounter,
		Profile:           rec.Profile,
	})
}

func (s *Server) handleCreateVoucher(w http.ResponseWriter, r *http.Request) {
	if !s.adminOK(r) {
		s.writeErr(w, 401, "unauthorized")
		return
	}
	var req CreateVoucherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErr(w, 400, "bad_request")
		return
	}
	maxUses := req.MaxUses
	if maxUses <= 0 {
		maxUses = 1
	}
	code := req.Code
	if code == "" {
		var err error
		code, err = GenerateVoucherCode()
		if err != nil {
			s.writeErr(w, 500, "internal")
			return
		}
	}
	idBytes := make([]byte, 8)
	_, _ = rand.Read(idBytes)
	id := hex.EncodeToString(idBytes)
	ttl := req.TTLHours
	if ttl == 0 {
		ttl = 168 // 7 days
	}
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(time.Duration(ttl) * time.Hour).Unix()
	}
	profile := req.Profile
	if profile == "" {
		profile = "strong"
	}
	v := &Voucher{
		ID:        id,
		CodeHash:  HashVoucherCode(code),
		Label:     req.Label,
		OrgID:     req.OrgID,
		Profile:   profile,
		MaxUses:   maxUses,
		ExpiresAt: exp,
		CreatedAt: time.Now().Unix(),
	}
	if err := s.Registry.AddVoucher(v); err != nil {
		s.writeErr(w, 500, "internal")
		return
	}
	s.Audit.Log(AuditEvent{Endpoint: "/v1/vouchers", ResultCode: 201, SrcIP: r.RemoteAddr, Error: "created:" + id})
	s.writeJSON(w, 201, CreateVoucherResponse{
		ID: id, Code: code, MaxUses: maxUses, ExpiresAt: exp,
		Label: req.Label, Profile: profile, OrgID: req.OrgID,
	})
}

func (s *Server) handleListVouchers(w http.ResponseWriter, r *http.Request) {
	if !s.adminOK(r) {
		s.writeErr(w, 401, "unauthorized")
		return
	}
	s.writeJSON(w, 200, map[string]any{"vouchers": s.Registry.ListVouchers()})
}

func (s *Server) handleRevokeVoucher(w http.ResponseWriter, r *http.Request) {
	if !s.adminOK(r) {
		s.writeErr(w, 401, "unauthorized")
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		s.writeErr(w, 400, "bad_request")
		return
	}
	if err := s.Registry.RevokeVoucher(body.ID); err != nil {
		s.writeErr(w, 404, "not_found")
		return
	}
	s.writeJSON(w, 200, map[string]any{"id": body.ID, "revoked": true})
}

func (s *Server) authenticateIssue(r *http.Request, req *IssueRequest) (deviceID string, rec *DeviceRecord, errCode string) {
	deviceID = strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return "", nil, "unauthorized"
	}
	if h := strings.TrimSpace(r.Header.Get("X-Device-ID")); h != "" && h != deviceID {
		return "", nil, "unauthorized"
	}
	rec = s.Registry.Get(deviceID)
	if rec == nil {
		return "", nil, "not_enrolled"
	}
	if rec.Revoked {
		return "", nil, "revoked"
	}
	profile := req.Profile
	if profile == "" {
		profile = rec.Profile
	}
	if profile == "" {
		profile = "strong"
	}
	req.Profile = profile

	require := s.RequireSignature
	if len(rec.SignPK) == ed25519.PublicKeySize {
		require = true
	}
	if require {
		if len(req.Signature) == 0 {
			return "", nil, "unauthorized"
		}
		if len(rec.SignPK) != ed25519.PublicKeySize {
			return "", nil, "unauthorized"
		}
		if !device.VerifyIssue(ed25519.PublicKey(rec.SignPK), deviceID, req.Nonce, req.ClientTime, []byte(profile), req.Signature) {
			return "", nil, "unauthorized"
		}
	}
	return deviceID, rec, ""
}

func (s *Server) handleIssue(w http.ResponseWriter, r *http.Request) {
	var req IssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErr(w, 400, "bad_request")
		return
	}
	deviceID, rec, errCode := s.authenticateIssue(r, &req)
	if errCode != "" {
		code := 401
		if errCode == "not_enrolled" || errCode == "revoked" {
			code = 403
		}
		s.writeErr(w, code, errCode)
		s.Audit.Log(AuditEvent{Endpoint: "/v1/key/issue", DeviceID: req.DeviceID, ResultCode: code, Error: errCode, SrcIP: r.RemoteAddr})
		return
	}

	now := device.Now().UTC()
	kaTime := now.Unix()
	if err := device.CheckSkew(req.ClientTime, kaTime); err != nil {
		s.writeErr(w, 400, "clock_skew")
		s.Audit.Log(AuditEvent{Endpoint: "/v1/key/issue", DeviceID: deviceID, ResultCode: 400, Error: "clock_skew", SrcIP: r.RemoteAddr})
		return
	}
	epochID := coecrypto.EpochID(now)
	if req.EpochHint != 0 {
		diff := int64(req.EpochHint) - int64(epochID)
		if diff < 0 {
			diff = -diff
		}
		if diff > 1 {
			s.writeErr(w, 400, "epoch_mismatch")
			return
		}
	}

	profile := req.Profile
	var generalKey []byte
	var err error
	if profile == "simple" {
		org := rec.OrgID
		if org == "" {
			org = "default"
		}
		generalKey, err = coecrypto.DeriveOrgDayKey(s.Master, epochID, org)
	} else {
		generalKey, err = coecrypto.DeriveGeneralKey(s.Master, epochID, rec.DeviceID, rec.EnrollmentCounter)
	}
	if err != nil {
		s.writeErr(w, 500, "internal")
		return
	}

	serial, err := s.Registry.MarkIssued(deviceID, epochID)
	if err != nil {
		s.writeErr(w, 403, err.Error())
		return
	}

	nb, na := coecrypto.EpochBounds(epochID)
	reqID := s.XO.RequestID()
	tag := s.XO.PublicTag()

	s.Audit.Log(AuditEvent{
		Endpoint:   "/v1/key/issue",
		DeviceID:   deviceID,
		EpochID:    epochID,
		KeySerial:  serial,
		RequestID:  hex.EncodeToString(reqID),
		ResultCode: 200,
		Profile:    profile,
		SrcIP:      r.RemoteAddr,
	})

	s.writeJSON(w, 200, IssueResponse{
		DeviceID:     deviceID,
		EpochID:      epochID,
		NotBefore:    nb,
		NotAfter:     na,
		GraceSeconds: uint64(coecrypto.DefaultGraceSeconds),
		KeySerial:    serial,
		RequestID:    reqID,
		PublicTag:    tag,
		GeneralKey:   generalKey,
		Profile:      profile,
		KATime:       kaTime,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.Header.Get("X-Device-ID"))
	if q := r.URL.Query().Get("device_id"); q != "" && s.adminOK(r) {
		deviceID = q
	}
	if deviceID == "" {
		s.writeErr(w, 401, "unauthorized")
		return
	}
	rec := s.Registry.Get(deviceID)
	if rec == nil {
		s.writeErr(w, 403, "not_enrolled")
		return
	}
	now := device.Now().UTC()
	epoch := coecrypto.EpochID(now)
	resp := StatusResponse{
		DeviceID:        deviceID,
		Revoked:         rec.Revoked,
		CurrentEpochID:  epoch,
		IssuedThisEpoch: rec.IssuedEpoch == epoch,
	}
	if rec.IssuedEpoch == epoch {
		resp.KeySerial = rec.LastSerial
		_, na := coecrypto.EpochBounds(epoch)
		resp.NotAfter = na
	}
	s.writeJSON(w, 200, resp)
}

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if !s.adminOK(r) {
		s.writeErr(w, 401, "unauthorized")
		return
	}
	var req RevokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErr(w, 400, "bad_request")
		return
	}
	if err := s.Registry.Revoke(req.DeviceID); err != nil {
		s.writeErr(w, 403, "not_enrolled")
		return
	}
	s.Audit.Log(AuditEvent{Endpoint: "/v1/revoke", DeviceID: req.DeviceID, ResultCode: 200, SrcIP: r.RemoteAddr})
	s.writeJSON(w, 200, map[string]any{"device_id": req.DeviceID, "revoked": true})
}
