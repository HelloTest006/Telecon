package agent

import (
	"encoding/json"
	"net"
	"net/http"
	"time"
)

// LocalAPI is the localhost control plane for desktop apps.
// Binds 127.0.0.1 only — not the message path to peers.
type LocalAPI struct {
	Hub      *Hub
	DeviceID string
	Token    string // optional Bearer for local apps
	Status   func() map[string]any
}

// Handler returns HTTP mux.
func (a *LocalAPI) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", a.auth(a.handleHealth))
	mux.HandleFunc("GET /v1/status", a.auth(a.handleStatus))
	mux.HandleFunc("GET /v1/peers", a.auth(a.handlePeersGet))
	mux.HandleFunc("POST /v1/peers", a.auth(a.handlePeersPost))
	mux.HandleFunc("POST /v1/sessions", a.auth(a.handleConnect))
	mux.HandleFunc("POST /v1/sessions/webrtc", a.auth(a.handleConnectWebRTC))
	mux.HandleFunc("GET /v1/sessions", a.auth(a.handleSessions))
	mux.HandleFunc("POST /v1/send", a.auth(a.handleSend))
	mux.HandleFunc("GET /v1/inbox", a.auth(a.handleInbox))
	return mux
}

// ListenAndServe binds addr (use 127.0.0.1:PORT).
func (a *LocalAPI) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	// refuse non-loopback if misconfigured
	return http.Serve(ln, a.Handler())
}

func (a *LocalAPI) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.Token != "" {
			if r.Header.Get("Authorization") != "Bearer "+a.Token {
				writeJSON(w, 401, map[string]string{"error": "unauthorized"})
				return
			}
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (a *LocalAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "device_id": a.DeviceID})
}

func (a *LocalAPI) handleStatus(w http.ResponseWriter, r *http.Request) {
	st := map[string]any{
		"device_id": a.DeviceID,
		"sessions":  a.Hub.Sessions(),
		"peers":     a.Hub.Peers(),
	}
	if a.Status != nil {
		for k, v := range a.Status() {
			st[k] = v
		}
	}
	writeJSON(w, 200, st)
}

func (a *LocalAPI) handlePeersGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.Hub.Peers())
}

func (a *LocalAPI) handlePeersPost(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DeviceID string `json:"device_id"`
		Addr     string `json:"addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.DeviceID == "" || body.Addr == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_request"})
		return
	}
	a.Hub.AddPeer(body.DeviceID, body.Addr)
	writeJSON(w, 200, map[string]string{"ok": "true", "device_id": body.DeviceID})
}

func (a *LocalAPI) handleConnect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DeviceID string `json:"device_id"`
		Addr     string `json:"addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.DeviceID == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_request"})
		return
	}
	if err := a.Hub.Connect(body.DeviceID, body.Addr); err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "peer_id": body.DeviceID})
}

func (a *LocalAPI) handleConnectWebRTC(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DeviceID  string `json:"device_id"`
		SignalURL string `json:"signal_url"`
		Role      string `json:"role"` // dial | accept
		WaitSec   int    `json:"wait_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.DeviceID == "" || body.SignalURL == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_request"})
		return
	}
	role := body.Role
	if role == "" {
		role = "dial"
	}
	wait := time.Duration(body.WaitSec) * time.Second
	if wait <= 0 {
		wait = 60 * time.Second
	}
	asDial := role != "accept"
	if err := a.Hub.ConnectWebRTC(body.DeviceID, body.SignalURL, asDial, wait); err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "peer_id": body.DeviceID, "transport": "webrtc", "role": role})
}

func (a *LocalAPI) handleSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.Hub.Sessions())
}

func (a *LocalAPI) handleSend(w http.ResponseWriter, r *http.Request) {
	var body struct {
		To   string `json:"to"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.To == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_request"})
		return
	}
	if err := a.Hub.Send(body.To, []byte(body.Text)); err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *LocalAPI) handleInbox(w http.ResponseWriter, r *http.Request) {
	wait := r.URL.Query().Get("wait")
	clear := r.URL.Query().Get("clear") != "0"
	if wait != "" {
		d, err := time.ParseDuration(wait)
		if err != nil {
			d = 5 * time.Second
		}
		if m, ok := a.Hub.WaitInbox(d); ok {
			writeJSON(w, 200, map[string]any{"messages": []InboxMsg{m}})
			return
		}
		writeJSON(w, 200, map[string]any{"messages": []InboxMsg{}})
		return
	}
	writeJSON(w, 200, map[string]any{"messages": a.Hub.Inbox(clear)})
}
