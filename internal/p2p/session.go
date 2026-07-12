package p2p

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	coecrypto "github.com/telecon/coe/internal/crypto"
	"github.com/telecon/coe/internal/device"
)

// LocalCreds are local device secrets for handshake.
type LocalCreds struct {
	DeviceID   string
	IdentitySK []byte
	IdentityPK []byte
	GeneralKey []byte
	KeySerial  uint64
	EpochID    uint64
	Profile    string
}

// SessionConfig configures a P2P session.
// PeerGeneralKey is optional (legacy); Strong profile exchanges keys via KEY_OFFER.
type SessionConfig struct {
	Local          LocalCreds
	PeerGeneralKey []byte // unused when KEY_OFFER path used
	PeerDeviceID   string
}

// Session is an established COE P2P session.
type Session struct {
	conn    net.Conn
	localID string
	peerID  string
	epochID uint64
	sendKey []byte
	recvKey []byte
	nextSeq uint64
	replay  *ReplayWindow
	mu      sync.Mutex
	closed  bool
}

// Dial connects as initiator over TCP.
func Dial(addr string, cfg SessionConfig) (*Session, error) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, err
	}
	s, err := handshakeInitiator(conn, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return s, nil
}

// DialConn runs initiator handshake on an existing conn (TCP or WebRTC data channel).
func DialConn(conn net.Conn, cfg SessionConfig) (*Session, error) {
	s, err := handshakeInitiator(conn, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return s, nil
}

// Accept runs responder handshake.
func Accept(conn net.Conn, cfg SessionConfig) (*Session, error) {
	return handshakeResponder(conn, cfg)
}

func randomNonce16() ([]byte, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	return b, err
}

func sealGeneralKey(wrapKey, generalKey []byte, epoch uint64, senderID string) (KeyOffer, error) {
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return KeyOffer{}, err
	}
	aad := coecrypto.Concat([]byte("COE-v1-keyoffer"), coecrypto.I2OSP(epoch, 8), []byte(senderID))
	ct, err := coecrypto.Seal(wrapKey, nonce, aad, generalKey)
	if err != nil {
		return KeyOffer{}, err
	}
	return KeyOffer{Type: TypeKeyOffer, EpochID: epoch, Nonce: nonce, Ciphertext: ct}, nil
}

func openGeneralKey(wrapKey []byte, offer KeyOffer, epoch uint64, senderID string) ([]byte, error) {
	aad := coecrypto.Concat([]byte("COE-v1-keyoffer"), coecrypto.I2OSP(epoch, 8), []byte(senderID))
	pt, err := coecrypto.Open(wrapKey, offer.Nonce, aad, offer.Ciphertext)
	if err != nil {
		return nil, err
	}
	if len(pt) != 32 {
		return nil, fmt.Errorf("bad general key length")
	}
	return pt, nil
}

func handshakeInitiator(conn net.Conn, cfg SessionConfig) (*Session, error) {
	lc := cfg.Local
	profile := lc.Profile
	if profile == "" {
		profile = "strong"
	}
	ephSK, ephPK, err := coecrypto.GenerateX25519()
	if err != nil {
		return nil, err
	}
	nonce, err := randomNonce16()
	if err != nil {
		return nil, err
	}
	hello := Hello{
		Type: TypeHello, Ver: 1, DeviceID: lc.DeviceID, EpochID: lc.EpochID,
		IdentityPK: lc.IdentityPK, EphPK: ephPK, SessionNonce: nonce,
		KeySerial: lc.KeySerial, Profile: profile,
	}
	if err := writeJSON(conn, hello); err != nil {
		return nil, err
	}
	var ack HelloAck
	if err := readJSON(conn, &ack); err != nil {
		return nil, err
	}
	if ack.Type != TypeHelloAck || !ack.Accept {
		return nil, fmt.Errorf("handshake rejected")
	}
	if cfg.PeerDeviceID != "" && ack.DeviceID != cfg.PeerDeviceID {
		return nil, fmt.Errorf("unexpected peer %s", ack.DeviceID)
	}
	agreed, ok := coecrypto.AgreeEpoch(hello.EpochID, ack.EpochID, device.Now(), coecrypto.DefaultGraceSeconds)
	if !ok {
		return nil, fmt.Errorf("epoch disagree %d vs %d", hello.EpochID, ack.EpochID)
	}

	var peerGK []byte
	if profile == "simple" {
		peerGK = lc.GeneralKey // same org day key
	} else {
		wrap, err := coecrypto.DeriveWrapKey(lc.IdentitySK, ack.IdentityPK, ephSK, ack.EphPK,
			nonce, ack.SessionNonce, agreed, lc.DeviceID, ack.DeviceID)
		if err != nil {
			return nil, err
		}
		offer, err := sealGeneralKey(wrap, lc.GeneralKey, agreed, lc.DeviceID)
		if err != nil {
			return nil, err
		}
		if err := writeJSON(conn, offer); err != nil {
			return nil, err
		}
		var peerOffer KeyOffer
		if err := readJSON(conn, &peerOffer); err != nil {
			return nil, err
		}
		if peerOffer.Type != TypeKeyOffer {
			return nil, fmt.Errorf("expected key offer")
		}
		peerGK, err = openGeneralKey(wrap, peerOffer, agreed, ack.DeviceID)
		if err != nil {
			return nil, fmt.Errorf("peer key offer: %w", err)
		}
	}

	params := coecrypto.SessionParams{
		EpochID: agreed, DeviceIDA: lc.DeviceID, DeviceIDB: ack.DeviceID,
		GeneralKeyA: lc.GeneralKey, GeneralKeyB: peerGK,
		IdentitySk: lc.IdentitySK, IdentityPkPeer: ack.IdentityPK,
		EphSk: ephSK, EphPkPeer: ack.EphPK,
		NonceA: nonce, NonceB: ack.SessionNonce, Profile: profile,
		LocalDeviceID: lc.DeviceID, PeerDeviceID: ack.DeviceID,
	}
	if profile == "simple" {
		params.OrgDayKey = lc.GeneralKey
	}
	_, sendKey, recvKey, err := coecrypto.DeriveSessionStrong(params)
	if err != nil {
		return nil, err
	}
	return &Session{
		conn: conn, localID: lc.DeviceID, peerID: ack.DeviceID,
		epochID: agreed, sendKey: sendKey, recvKey: recvKey,
		replay: NewReplayWindow(128),
	}, nil
}

func handshakeResponder(conn net.Conn, cfg SessionConfig) (*Session, error) {
	lc := cfg.Local
	profile := lc.Profile
	if profile == "" {
		profile = "strong"
	}
	var hello Hello
	if err := readJSON(conn, &hello); err != nil {
		return nil, err
	}
	if hello.Type != TypeHello || hello.Ver != 1 {
		return nil, fmt.Errorf("bad hello")
	}
	if cfg.PeerDeviceID != "" && hello.DeviceID != cfg.PeerDeviceID {
		_ = writeJSON(conn, HelloAck{Type: TypeHelloAck, Accept: false, DeviceID: lc.DeviceID})
		return nil, fmt.Errorf("unexpected peer")
	}
	ephSK, ephPK, err := coecrypto.GenerateX25519()
	if err != nil {
		return nil, err
	}
	nonce, err := randomNonce16()
	if err != nil {
		return nil, err
	}
	ack := HelloAck{
		Type: TypeHelloAck, Ver: 1, DeviceID: lc.DeviceID, EpochID: lc.EpochID,
		IdentityPK: lc.IdentityPK, EphPK: ephPK, SessionNonce: nonce,
		KeySerial: lc.KeySerial, Profile: profile, Accept: true,
	}
	agreed, ok := coecrypto.AgreeEpoch(hello.EpochID, ack.EpochID, device.Now(), coecrypto.DefaultGraceSeconds)
	if !ok {
		ack.Accept = false
		_ = writeJSON(conn, ack)
		return nil, fmt.Errorf("epoch disagree")
	}
	if err := writeJSON(conn, ack); err != nil {
		return nil, err
	}

	var peerGK []byte
	if profile == "simple" {
		peerGK = lc.GeneralKey
	} else {
		wrap, err := coecrypto.DeriveWrapKey(lc.IdentitySK, hello.IdentityPK, ephSK, hello.EphPK,
			hello.SessionNonce, nonce, agreed, hello.DeviceID, lc.DeviceID)
		if err != nil {
			return nil, err
		}
		// initiator sends first
		var peerOffer KeyOffer
		if err := readJSON(conn, &peerOffer); err != nil {
			return nil, err
		}
		if peerOffer.Type != TypeKeyOffer {
			return nil, fmt.Errorf("expected key offer")
		}
		peerGK, err = openGeneralKey(wrap, peerOffer, agreed, hello.DeviceID)
		if err != nil {
			return nil, fmt.Errorf("peer key offer: %w", err)
		}
		offer, err := sealGeneralKey(wrap, lc.GeneralKey, agreed, lc.DeviceID)
		if err != nil {
			return nil, err
		}
		if err := writeJSON(conn, offer); err != nil {
			return nil, err
		}
	}

	params := coecrypto.SessionParams{
		EpochID: agreed, DeviceIDA: hello.DeviceID, DeviceIDB: lc.DeviceID,
		GeneralKeyA: peerGK, GeneralKeyB: lc.GeneralKey,
		IdentitySk: lc.IdentitySK, IdentityPkPeer: hello.IdentityPK,
		EphSk: ephSK, EphPkPeer: hello.EphPK,
		NonceA: hello.SessionNonce, NonceB: nonce, Profile: profile,
		LocalDeviceID: lc.DeviceID, PeerDeviceID: hello.DeviceID,
	}
	if profile == "simple" {
		params.OrgDayKey = lc.GeneralKey
	}
	_, sendKey, recvKey, err := coecrypto.DeriveSessionStrong(params)
	if err != nil {
		return nil, err
	}
	return &Session{
		conn: conn, localID: lc.DeviceID, peerID: hello.DeviceID,
		epochID: agreed, sendKey: sendKey, recvKey: recvKey,
		replay: NewReplayWindow(128),
	}, nil
}

func writeJSON(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return WriteFrame(w, b)
}

func readJSON(r io.Reader, v any) error {
	b, err := ReadFrame(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (s *Session) Send(plaintext []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("session closed")
	}
	seq := s.nextSeq
	s.nextSeq++
	nonce := coecrypto.NonceFromSeq(seq)
	aad := coecrypto.BuildAAD(1, s.epochID, s.localID, s.peerID, seq)
	ct, err := coecrypto.Seal(s.sendKey, nonce, aad, plaintext)
	if err != nil {
		return err
	}
	return writeJSON(s.conn, DataMsg{
		Type: TypeData, EpochID: s.epochID, Seq: seq, Nonce: nonce, Ciphertext: ct,
	})
}

func (s *Session) Recv() ([]byte, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fmt.Errorf("session closed")
	}
	s.mu.Unlock()

	b, err := ReadFrame(s.conn)
	if err != nil {
		return nil, err
	}
	var head struct {
		Type uint8 `json:"type"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	if head.Type == TypeClose {
		var cm CloseMsg
		_ = json.Unmarshal(b, &cm)
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		return nil, fmt.Errorf("peer closed: %s", cm.Reason)
	}
	if head.Type != TypeData {
		return nil, fmt.Errorf("unexpected type %d", head.Type)
	}
	var msg DataMsg
	if err := json.Unmarshal(b, &msg); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if msg.EpochID != s.epochID {
		return nil, fmt.Errorf("epoch mismatch")
	}
	if !s.replay.Accept(msg.Seq) {
		return nil, fmt.Errorf("replay or old seq %d", msg.Seq)
	}
	nonce := msg.Nonce
	if len(nonce) == 0 {
		nonce = coecrypto.NonceFromSeq(msg.Seq)
	}
	aad := coecrypto.BuildAAD(1, s.epochID, s.peerID, s.localID, msg.Seq)
	return coecrypto.Open(s.recvKey, nonce, aad, msg.Ciphertext)
}

func (s *Session) Close(reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	_ = writeJSON(s.conn, CloseMsg{Type: TypeClose, Reason: reason})
	return s.conn.Close()
}

func (s *Session) Epoch() uint64   { return s.epochID }
func (s *Session) PeerID() string  { return s.peerID }
func (s *Session) LocalID() string { return s.localID }

func (s *Session) SetDeadline(t time.Time) error {
	return s.conn.SetDeadline(t)
}

func (s *Session) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}
