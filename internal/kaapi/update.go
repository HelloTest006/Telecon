package kaapi

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
)

// UpdateManifest is served at GET /v1/update/check for agents.
// Unsigned for beta; production should sign this file (roadmap).
type UpdateManifest struct {
	Latest     string            `json:"latest"`
	MinVersion string            `json:"min_version,omitempty"`
	Notes      string            `json:"notes,omitempty"`
	Channels   map[string]string `json:"channels,omitempty"` // e.g. windows-amd64 -> URL
	// optional: sha256 per channel later
}

// UpdateHub holds optional on-disk manifest for agents.
type UpdateHub struct {
	mu       sync.RWMutex
	path     string
	manifest UpdateManifest
}

// LoadUpdateManifest reads JSON from path; empty path disables checks.
func LoadUpdateManifest(path string) (*UpdateHub, error) {
	h := &UpdateHub{path: path}
	if path == "" {
		return h, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return h, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, &h.manifest); err != nil {
		return nil, err
	}
	return h, nil
}

// Reload re-reads file from disk.
func (h *UpdateHub) Reload() error {
	if h == nil || h.path == "" {
		return nil
	}
	b, err := os.ReadFile(h.path)
	if err != nil {
		return err
	}
	var m UpdateManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	h.mu.Lock()
	h.manifest = m
	h.mu.Unlock()
	return nil
}

func (h *UpdateHub) snapshot() UpdateManifest {
	if h == nil {
		return UpdateManifest{}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.manifest
}

// UpdateCheckRequest is optional query: ?current=0.1.0-beta&os=windows&arch=amd64
type UpdateCheckResponse struct {
	UpdateAvailable bool   `json:"update_available"`
	Current         string `json:"current,omitempty"`
	Latest          string `json:"latest,omitempty"`
	MinVersion      string `json:"min_version,omitempty"`
	Notes           string `json:"notes,omitempty"`
	DownloadURL     string `json:"download_url,omitempty"`
	Forced          bool   `json:"forced,omitempty"` // current < min_version (string compare only in beta)
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	cur := r.URL.Query().Get("current")
	osName := strings.ToLower(r.URL.Query().Get("os"))
	arch := strings.ToLower(r.URL.Query().Get("arch"))
	m := UpdateManifest{}
	if s.Updates != nil {
		_ = s.Updates.Reload()
		m = s.Updates.snapshot()
	}
	if m.Latest == "" {
		s.writeJSON(w, 200, UpdateCheckResponse{UpdateAvailable: false, Current: cur})
		return
	}
	channel := osName + "-" + arch
	url := ""
	if m.Channels != nil {
		url = m.Channels[channel]
		if url == "" {
			url = m.Channels[osName]
		}
	}
	avail := cur != "" && cur != m.Latest
	forced := m.MinVersion != "" && cur != "" && cur < m.MinVersion // beta: lexicographic only
	s.writeJSON(w, 200, UpdateCheckResponse{
		UpdateAvailable: avail || forced,
		Current:         cur,
		Latest:          m.Latest,
		MinVersion:      m.MinVersion,
		Notes:           m.Notes,
		DownloadURL:     url,
		Forced:          forced,
	})
}
