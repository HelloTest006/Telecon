package kaapi

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// AuditEvent is a non-secret log line.
type AuditEvent struct {
	Timestamp  string `json:"timestamp"`
	RequestID  string `json:"request_id,omitempty"`
	DeviceID   string `json:"device_id,omitempty"`
	EpochID    uint64 `json:"epoch_id,omitempty"`
	KeySerial  uint64 `json:"key_serial,omitempty"`
	Endpoint   string `json:"endpoint"`
	SrcIP      string `json:"src_ip,omitempty"`
	ResultCode int    `json:"result_code"`
	Profile    string `json:"profile,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Auditor writes JSON lines.
type Auditor struct {
	w  io.Writer
	mu sync.Mutex
}

// NewAuditor creates auditor writing to w (e.g. os.Stdout or file).
func NewAuditor(w io.Writer) *Auditor {
	if w == nil {
		w = os.Stdout
	}
	return &Auditor{w: w}
}

// Log writes one event. Never pass general keys.
func (a *Auditor) Log(ev AuditEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if ev.Timestamp == "" {
		ev.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	b, _ := json.Marshal(ev)
	_, _ = a.w.Write(append(b, '\n'))
}
