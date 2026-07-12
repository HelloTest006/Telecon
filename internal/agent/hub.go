package agent

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/telecon/coe/internal/config"
	"github.com/telecon/coe/internal/p2p"
	"github.com/telecon/coe/internal/webrtcx"
)

// InboxMsg is a received application message.
type InboxMsg struct {
	From      string    `json:"from"`
	Body      string    `json:"body"`
	Raw       []byte    `json:"-"`
	Received  time.Time `json:"received"`
	SessionOK bool      `json:"-"`
}

// SessionInfo is public session status.
type SessionInfo struct {
	PeerID  string `json:"peer_id"`
	EpochID uint64 `json:"epoch_id"`
	Active  bool   `json:"active"`
}

// ICEConfig is STUN/TURN for WebRTC.
type ICEConfig struct {
	STUN     []string
	TURNURLs []string
	TURNUser string
	TURNPass string
}

// Hub manages P2P sessions for the desktop agent.
type Hub struct {
	mu       sync.Mutex
	creds    p2p.LocalCreds
	peers    map[string]string // device_id -> addr
	sessions map[string]*p2p.Session
	inbox    []InboxMsg
	inboxCh  chan InboxMsg
	logf     func(string, ...any)
	ice      ICEConfig
}

// NewHub creates a hub with local credentials.
func NewHub(creds p2p.LocalCreds) *Hub {
	return &Hub{
		creds:    creds,
		peers:    make(map[string]string),
		sessions: make(map[string]*p2p.Session),
		inboxCh:  make(chan InboxMsg, 64),
		logf:     log.Printf,
	}
}

// SetICE configures STUN/TURN for WebRTC sessions.
func (h *Hub) SetICE(ice ICEConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ice = ice
}

// SetCreds updates daily key material after re-issue.
func (h *Hub) SetCreds(c p2p.LocalCreds) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.creds = c
}

// Creds returns a copy of local creds.
func (h *Hub) Creds() p2p.LocalCreds {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.creds
}

// AddPeer registers roster entry.
func (h *Hub) AddPeer(deviceID, addr string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.peers[deviceID] = addr
}

// Peers returns roster copy.
func (h *Hub) Peers() map[string]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]string, len(h.peers))
	for k, v := range h.peers {
		out[k] = v
	}
	return out
}

// LoadPeers from config.
func (h *Hub) LoadPeers(peers []config.Peer) {
	for _, p := range peers {
		if p.DeviceID != "" && p.Addr != "" {
			h.AddPeer(p.DeviceID, p.Addr)
		}
	}
}

// Sessions lists active sessions.
func (h *Hub) Sessions() []SessionInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]SessionInfo, 0, len(h.sessions))
	for id, s := range h.sessions {
		out = append(out, SessionInfo{PeerID: id, EpochID: s.Epoch(), Active: true})
	}
	return out
}

// ConnectWebRTC dials peer over WebRTC data channel via signaling server.
// signalURL example: http://127.0.0.1:8450 — carries SDP/ICE only, not app data.
func (h *Hub) ConnectWebRTC(peerID, signalURL string, asInitiator bool, wait time.Duration) error {
	if wait <= 0 {
		wait = 60 * time.Second
	}
	h.mu.Lock()
	creds := h.creds
	ice := h.ice
	if old, ok := h.sessions[peerID]; ok {
		_ = old.Close("reconnect")
		delete(h.sessions, peerID)
	}
	localID := creds.DeviceID
	h.mu.Unlock()

	wcfg := webrtcx.Config{
		SignalURL: signalURL,
		LocalID:   localID,
		PeerID:    peerID,
		STUN:      ice.STUN,
		TURNURLs:  ice.TURNURLs,
		TURNUser:  ice.TURNUser,
		TURNPass:  ice.TURNPass,
	}
	var (
		conn net.Conn
		err  error
	)
	if asInitiator {
		conn, err = webrtcx.Dial(wcfg)
	} else {
		conn, err = webrtcx.Accept(wcfg, wait)
	}
	if err != nil {
		return err
	}

	var sess *p2p.Session
	if asInitiator {
		sess, err = p2p.DialConn(conn, p2p.SessionConfig{Local: creds, PeerDeviceID: peerID})
	} else {
		sess, err = p2p.Accept(conn, p2p.SessionConfig{Local: creds, PeerDeviceID: peerID})
	}
	if err != nil {
		_ = conn.Close()
		return err
	}
	h.mu.Lock()
	h.sessions[peerID] = sess
	h.mu.Unlock()
	go h.readLoop(sess)
	h.logf("webrtc session with %s epoch=%d initiator=%v", peerID, sess.Epoch(), asInitiator)
	return nil
}

// Connect dials peer by id (must be in roster) or addr override.
func (h *Hub) Connect(peerID, addr string) error {
	h.mu.Lock()
	if addr == "" {
		addr = h.peers[peerID]
	} else {
		h.peers[peerID] = addr
	}
	creds := h.creds
	if old, ok := h.sessions[peerID]; ok {
		_ = old.Close("reconnect")
		delete(h.sessions, peerID)
	}
	h.mu.Unlock()

	if addr == "" {
		return fmt.Errorf("no address for peer %s", peerID)
	}
	sess, err := p2p.Dial(addr, p2p.SessionConfig{
		Local:        creds,
		PeerDeviceID: peerID,
	})
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.sessions[peerID] = sess
	h.mu.Unlock()
	go h.readLoop(sess)
	h.logf("session up with %s epoch=%d", peerID, sess.Epoch())
	return nil
}

// AcceptConn completes handshake on inbound TCP and registers session.
func (h *Hub) AcceptConn(conn net.Conn) error {
	h.mu.Lock()
	creds := h.creds
	h.mu.Unlock()
	sess, err := p2p.Accept(conn, p2p.SessionConfig{Local: creds})
	if err != nil {
		_ = conn.Close()
		return err
	}
	h.mu.Lock()
	if old, ok := h.sessions[sess.PeerID()]; ok {
		_ = old.Close("replaced")
	}
	h.sessions[sess.PeerID()] = sess
	h.mu.Unlock()
	go h.readLoop(sess)
	h.logf("accepted session from %s", sess.PeerID())
	return nil
}

func (h *Hub) readLoop(sess *p2p.Session) {
	peer := sess.PeerID()
	for {
		pt, err := sess.Recv()
		if err != nil {
			h.logf("session %s closed: %v", peer, err)
			h.mu.Lock()
			if h.sessions[peer] == sess {
				delete(h.sessions, peer)
			}
			h.mu.Unlock()
			_ = sess.Close("recv-err")
			return
		}
		msg := InboxMsg{
			From:     peer,
			Body:     string(pt),
			Raw:      append([]byte(nil), pt...),
			Received: time.Now().UTC(),
		}
		h.mu.Lock()
		h.inbox = append(h.inbox, msg)
		if len(h.inbox) > 500 {
			h.inbox = h.inbox[len(h.inbox)-500:]
		}
		h.mu.Unlock()
		select {
		case h.inboxCh <- msg:
		default:
		}
	}
}

// Send delivers plaintext to peer (must have active session).
func (h *Hub) Send(peerID string, body []byte) error {
	h.mu.Lock()
	sess := h.sessions[peerID]
	h.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("no session with %s", peerID)
	}
	return sess.Send(body)
}

// Inbox returns and optionally clears buffered messages.
func (h *Hub) Inbox(clear bool) []InboxMsg {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := append([]InboxMsg(nil), h.inbox...)
	if clear {
		h.inbox = nil
	}
	return out
}

// WaitInbox waits up to d for a message.
func (h *Hub) WaitInbox(d time.Duration) (InboxMsg, bool) {
	select {
	case m := <-h.inboxCh:
		return m, true
	case <-time.After(d):
		return InboxMsg{}, false
	}
}

// CloseAll tears down sessions.
func (h *Hub) CloseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, s := range h.sessions {
		_ = s.Close("shutdown")
		delete(h.sessions, id)
	}
}
