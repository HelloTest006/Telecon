package signal

import (
	"encoding/json"
	"sync"
	"time"
)

// Hub is an in-memory signaling exchange (not for multi-host prod without sticky/redis).
type Hub struct {
	mu    sync.Mutex
	rooms map[string]*room
}

type room struct {
	// queues per device id
	q map[string]chan Envelope
}

// NewHub creates a signaling hub.
func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*room)}
}

func (h *Hub) room(id string) *room {
	r, ok := h.rooms[id]
	if !ok {
		r = &room{q: make(map[string]chan Envelope)}
		h.rooms[id] = r
	}
	return r
}

func (h *Hub) ensureQueue(r *room, deviceID string) chan Envelope {
	ch, ok := r.q[deviceID]
	if !ok {
		ch = make(chan Envelope, 32)
		r.q[deviceID] = ch
	}
	return ch
}

// Publish sends a message to the peer in the room.
func (h *Hub) Publish(env Envelope) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	roomID := env.Room
	if roomID == "" && env.To != "" && env.From != "" {
		roomID = RoomID(env.From, env.To)
		env.Room = roomID
	}
	r := h.room(roomID)
	target := env.To
	if target == "" {
		// broadcast to other queues
		for id, ch := range r.q {
			if id == env.From {
				continue
			}
			select {
			case ch <- env:
			default:
			}
		}
		return nil
	}
	ch := h.ensureQueue(r, target)
	select {
	case ch <- env:
		return nil
	default:
		return errFull
	}
}

// Subscribe registers and waits for next message for device in room.
func (h *Hub) Subscribe(roomID, deviceID string, wait time.Duration) (Envelope, bool) {
	h.mu.Lock()
	r := h.room(roomID)
	ch := h.ensureQueue(r, deviceID)
	h.mu.Unlock()

	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case env := <-ch:
		return env, true
	case <-t.C:
		return Envelope{}, false
	}
}

// Drain non-blocking poll.
func (h *Hub) Drain(roomID, deviceID string, max int) []Envelope {
	h.mu.Lock()
	r := h.room(roomID)
	ch := h.ensureQueue(r, deviceID)
	h.mu.Unlock()
	var out []Envelope
	for i := 0; i < max; i++ {
		select {
		case env := <-ch:
			out = append(out, env)
		default:
			return out
		}
	}
	return out
}

var errFull = &fullErr{}

type fullErr struct{}

func (e *fullErr) Error() string { return "signal queue full" }

// Encode helper.
func Encode(env Envelope) []byte {
	b, _ := json.Marshal(env)
	return b
}
