package config

import (
	"encoding/json"
	"os"
)

// Peer is a roster entry.
type Peer struct {
	DeviceID string `json:"device_id"`
	Addr     string `json:"addr"`
}

// NodeConfig is coe-node / desktop agent configuration.
type NodeConfig struct {
	DeviceID     string   `json:"device_id"`
	IdentityPath string   `json:"identity_path"`
	StoreDir     string   `json:"store_dir"`
	KAURL        string   `json:"ka_url"`
	KACAFile     string   `json:"ka_ca_file"`
	KAInsecure   bool     `json:"ka_insecure"`
	ListenAddr   string   `json:"listen_addr"`
	APIAddr      string   `json:"api_addr"`
	APIToken     string   `json:"api_token,omitempty"`
	Peers        []Peer   `json:"peers"`
	Profile      string   `json:"profile"`
	AdminToken   string   `json:"admin_token,omitempty"`
	VoucherCode  string   `json:"voucher_code,omitempty"` // enroll only; do not keep long-term
	STUN         []string `json:"stun,omitempty"`
	TURNURLs     []string `json:"turn_urls,omitempty"`
	TURNUser     string   `json:"turn_user,omitempty"`
	TURNPass     string   `json:"turn_pass,omitempty"`
	SignalURL    string   `json:"signal_url,omitempty"`
}

// LoadNode reads JSON config.
func LoadNode(path string) (NodeConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return NodeConfig{}, err
	}
	var c NodeConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return NodeConfig{}, err
	}
	if c.Profile == "" {
		c.Profile = "strong"
	}
	return c, nil
}

// SaveNode writes JSON config.
func SaveNode(path string, c NodeConfig) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
