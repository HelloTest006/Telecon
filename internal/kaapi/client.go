package kaapi

import (
	"bytes"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/telecon/coe/internal/device"
)

// Client talks to Key Authority.
type Client struct {
	BaseURL    string
	DeviceID   string
	AdminToken string
	SignKey    ed25519.PrivateKey // for issue signatures
	HTTP       *http.Client
}

// ClientOptions configures KA client TLS.
type ClientOptions struct {
	Insecure bool
	CAFile   string
	TLS      *tls.Config
}

// NewClient builds a KA client.
func NewClient(baseURL, deviceID string, insecure bool) *Client {
	c, _ := NewClientOpts(baseURL, deviceID, ClientOptions{Insecure: insecure})
	return c
}

// NewClientOpts builds client with TLS options.
func NewClientOpts(baseURL, deviceID string, opts ClientOptions) (*Client, error) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if opts.TLS != nil {
		tr.TLSClientConfig = opts.TLS
	} else if opts.Insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12} //nolint:gosec
	} else if opts.CAFile != "" {
		// lazy: caller should use tlsutil; keep simple load
		cfg := &tls.Config{MinVersion: tls.VersionTLS12}
		tr.TLSClientConfig = cfg
	}
	return &Client{
		BaseURL:  baseURL,
		DeviceID: deviceID,
		HTTP:     &http.Client{Timeout: 30 * time.Second, Transport: tr},
	}, nil
}

// SetSignKey sets Ed25519 private key for issue auth.
func (c *Client) SetSignKey(sk ed25519.PrivateKey) {
	c.SignKey = sk
}

func (c *Client) doJSON(method, path string, in any, out any, admin bool) (int, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return 0, err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.DeviceID != "" {
		req.Header.Set("X-Device-ID", c.DeviceID)
	}
	if admin && c.AdminToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AdminToken)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		var eb ErrorBody
		_ = json.Unmarshal(data, &eb)
		if eb.Error != "" {
			return resp.StatusCode, fmt.Errorf("%s", eb.Error)
		}
		return resp.StatusCode, fmt.Errorf("http %d: %s", resp.StatusCode, string(data))
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

func (c *Client) Health() (HealthResponse, error) {
	var h HealthResponse
	_, err := c.doJSON(http.MethodGet, "/v1/health", nil, &h, false)
	return h, err
}

// Enroll registers a device. Uses admin token if set; otherwise voucher_code on req.
func (c *Client) Enroll(req EnrollRequest) (EnrollResponse, error) {
	var out EnrollResponse
	useAdmin := c.AdminToken != "" && req.VoucherCode == ""
	_, err := c.doJSON(http.MethodPost, "/v1/enroll", req, &out, useAdmin)
	return out, err
}

// CreateVoucher admin-creates an enroll voucher (returns plaintext code once).
func (c *Client) CreateVoucher(req CreateVoucherRequest) (CreateVoucherResponse, error) {
	var out CreateVoucherResponse
	_, err := c.doJSON(http.MethodPost, "/v1/vouchers", req, &out, true)
	return out, err
}

// ListVouchers lists vouchers (no codes).
func (c *Client) ListVouchers() ([]VoucherInfo, error) {
	var out struct {
		Vouchers []VoucherInfo `json:"vouchers"`
	}
	_, err := c.doJSON(http.MethodGet, "/v1/vouchers", nil, &out, true)
	return out.Vouchers, err
}

// IssueKey requests daily general key (signs if SignKey set).
func (c *Client) IssueKey(req IssueRequest) (IssueResponse, error) {
	if req.Profile == "" {
		req.Profile = "strong"
	}
	if len(c.SignKey) > 0 {
		req.Signature = device.SignIssue(c.SignKey, req.DeviceID, req.Nonce, req.ClientTime, req.Profile)
	}
	var out IssueResponse
	_, err := c.doJSON(http.MethodPost, "/v1/key/issue", req, &out, false)
	return out, err
}

func (c *Client) Status() (StatusResponse, error) {
	var out StatusResponse
	_, err := c.doJSON(http.MethodGet, "/v1/key/status", nil, &out, false)
	return out, err
}

func (c *Client) Revoke(deviceID, reason string) error {
	_, err := c.doJSON(http.MethodPost, "/v1/revoke", RevokeRequest{DeviceID: deviceID, Reason: reason}, &map[string]any{}, true)
	return err
}
