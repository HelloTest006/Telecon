package signal

import (
	"encoding/json"
	"net/http"
	"time"
)

// Server is HTTP long-poll signaling (no app messages).
type Server struct {
	Hub *Hub
}

// Handler mounts /v1/signal/*
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/signal/send", s.handleSend)
	mux.HandleFunc("GET /v1/signal/poll", s.handlePoll)
	mux.HandleFunc("GET /v1/signal/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","role":"signal"}`))
	})
	return mux
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	var env Envelope
	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		http.Error(w, `{"error":"bad_request"}`, 400)
		return
	}
	if env.From == "" || env.Type == "" {
		http.Error(w, `{"error":"bad_request"}`, 400)
		return
	}
	if env.Room == "" && env.To != "" {
		env.Room = RoomID(env.From, env.To)
	}
	if env.Room == "" {
		http.Error(w, `{"error":"room_or_to_required"}`, 400)
		return
	}
	if err := s.Hub.Publish(env); err != nil {
		http.Error(w, `{"error":"queue_full"}`, 503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handlePoll(w http.ResponseWriter, r *http.Request) {
	room := r.URL.Query().Get("room")
	from := r.URL.Query().Get("from")
	waitStr := r.URL.Query().Get("wait")
	if room == "" || from == "" {
		http.Error(w, `{"error":"room_and_from_required"}`, 400)
		return
	}
	wait := 25 * time.Second
	if waitStr != "" {
		if d, err := time.ParseDuration(waitStr); err == nil && d > 0 && d < 60*time.Second {
			wait = d
		}
	}
	// first drain any queued
	msgs := s.Hub.Drain(room, from, 16)
	if len(msgs) == 0 {
		if env, ok := s.Hub.Subscribe(room, from, wait); ok {
			msgs = append(msgs, env)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"messages": msgs})
}
