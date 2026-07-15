package webrtcx

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/telecon/coe/internal/signal"
)

// Config for WebRTC sessions.
type Config struct {
	SignalURL string
	LocalID   string
	PeerID    string
	STUN      []string
	// TURN (relay) for symmetric NAT — optional
	TURNURLs []string
	TURNUser string
	TURNPass string
	// Policy: all (default) or relay (TURN-only when TURN configured)
	Policy ICEPolicy
}

func newPC(cfg Config) (*webrtc.PeerConnection, error) {
	ApplyEnv(&cfg)
	return webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers:         buildICEServers(cfg),
		ICETransportPolicy: iceTransportPolicy(cfg),
	})
}

// Dial initiates WebRTC to peer via signaling (this side creates offer).
func Dial(cfg Config) (*DCConn, error) {
	ApplyEnv(&cfg)
	sc := signal.NewClient(cfg.SignalURL, cfg.LocalID)
	room := signal.RoomID(cfg.LocalID, cfg.PeerID)

	pc, err := newPC(cfg)
	if err != nil {
		return nil, err
	}

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		b, _ := json.Marshal(c.ToJSON())
		_ = sc.Send(signal.Envelope{
			Type: "ice", From: cfg.LocalID, To: cfg.PeerID, Room: room, ICE: string(b),
		})
	})

	ordered := true
	dc, err := pc.CreateDataChannel("coe", &webrtc.DataChannelInit{Ordered: &ordered})
	if err != nil {
		_ = pc.Close()
		return nil, err
	}
	connCh := make(chan *DCConn, 1)
	dc.OnOpen(func() {
		connCh <- NewDCConn(pc, dc)
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		_ = pc.Close()
		return nil, err
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		_ = pc.Close()
		return nil, err
	}
	<-webrtc.GatheringCompletePromise(pc)
	offer = *pc.LocalDescription()

	if err := sc.Send(signal.Envelope{
		Type: "offer", From: cfg.LocalID, To: cfg.PeerID, Room: room, SDP: offer.SDP,
	}); err != nil {
		_ = pc.Close()
		return nil, err
	}

	deadline := time.Now().Add(45 * time.Second)
	var gotAnswer bool
	for time.Now().Before(deadline) {
		select {
		case c := <-connCh:
			return c, nil
		default:
		}
		msgs, err := sc.Poll(room, 3*time.Second)
		if err != nil {
			continue
		}
		for _, m := range msgs {
			switch m.Type {
			case "answer":
				if gotAnswer {
					continue
				}
				if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: m.SDP}); err != nil {
					_ = pc.Close()
					return nil, err
				}
				gotAnswer = true
			case "ice":
				var cand webrtc.ICECandidateInit
				if err := json.Unmarshal([]byte(m.ICE), &cand); err == nil {
					_ = pc.AddICECandidate(cand)
				}
			}
		}
	}
	select {
	case c := <-connCh:
		return c, nil
	default:
		_ = pc.Close()
		return nil, fmt.Errorf("webrtc dial timeout (try TURN if behind symmetric NAT)")
	}
}

// Accept waits for offer from peer and answers.
func Accept(cfg Config, wait time.Duration) (*DCConn, error) {
	ApplyEnv(&cfg)
	sc := signal.NewClient(cfg.SignalURL, cfg.LocalID)
	room := signal.RoomID(cfg.LocalID, cfg.PeerID)

	pc, err := newPC(cfg)
	if err != nil {
		return nil, err
	}
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		b, _ := json.Marshal(c.ToJSON())
		_ = sc.Send(signal.Envelope{
			Type: "ice", From: cfg.LocalID, To: cfg.PeerID, Room: room, ICE: string(b),
		})
	})

	connCh := make(chan *DCConn, 1)
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		dc.OnOpen(func() {
			connCh <- NewDCConn(pc, dc)
		})
	})

	deadline := time.Now().Add(wait)
	var gotOffer bool
	for time.Now().Before(deadline) {
		select {
		case c := <-connCh:
			return c, nil
		default:
		}
		msgs, err := sc.Poll(room, 3*time.Second)
		if err != nil {
			continue
		}
		for _, m := range msgs {
			if m.From == cfg.LocalID {
				continue
			}
			switch m.Type {
			case "offer":
				if gotOffer {
					continue
				}
				if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: m.SDP}); err != nil {
					_ = pc.Close()
					return nil, err
				}
				answer, err := pc.CreateAnswer(nil)
				if err != nil {
					_ = pc.Close()
					return nil, err
				}
				if err := pc.SetLocalDescription(answer); err != nil {
					_ = pc.Close()
					return nil, err
				}
				<-webrtc.GatheringCompletePromise(pc)
				answer = *pc.LocalDescription()
				if err := sc.Send(signal.Envelope{
					Type: "answer", From: cfg.LocalID, To: cfg.PeerID, Room: room, SDP: answer.SDP,
				}); err != nil {
					_ = pc.Close()
					return nil, err
				}
				gotOffer = true
			case "ice":
				var cand webrtc.ICECandidateInit
				if err := json.Unmarshal([]byte(m.ICE), &cand); err == nil {
					_ = pc.AddICECandidate(cand)
				}
			}
		}
	}
	select {
	case c := <-connCh:
		return c, nil
	default:
		_ = pc.Close()
		return nil, fmt.Errorf("webrtc accept timeout (try TURN if behind symmetric NAT)")
	}
}
