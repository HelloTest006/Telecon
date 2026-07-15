package webrtcx

import (
	"os"
	"strings"

	"github.com/pion/webrtc/v4"
)

// ICEPolicy controls candidate preference.
type ICEPolicy string

const (
	// ICEPolicyAll uses host/srflx/relay (default).
	ICEPolicyAll ICEPolicy = "all"
	// ICEPolicyRelay forces TURN only (hard NAT / corporate).
	ICEPolicyRelay ICEPolicy = "relay"
)

// ApplyEnv fills empty ICE fields from environment and policy.
// COE_STUN, COE_TURN_URLS, COE_TURN_USER, COE_TURN_PASS, COE_ICE_POLICY=all|relay
func ApplyEnv(cfg *Config) {
	if cfg == nil {
		return
	}
	if len(cfg.STUN) == 0 {
		if v := os.Getenv("COE_STUN"); v != "" {
			cfg.STUN = splitCSV(v)
		}
	}
	if len(cfg.TURNURLs) == 0 {
		if v := os.Getenv("COE_TURN_URLS"); v != "" {
			cfg.TURNURLs = splitCSV(v)
		}
	}
	if cfg.TURNUser == "" {
		cfg.TURNUser = os.Getenv("COE_TURN_USER")
	}
	if cfg.TURNPass == "" {
		cfg.TURNPass = os.Getenv("COE_TURN_PASS")
	}
	if cfg.Policy == "" {
		p := strings.ToLower(strings.TrimSpace(os.Getenv("COE_ICE_POLICY")))
		if p == "relay" || p == "turn" {
			cfg.Policy = ICEPolicyRelay
		} else {
			cfg.Policy = ICEPolicyAll
		}
	}
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// defaultSTUNList returns STUN URLs; multiple public servers for resilience.
func defaultSTUNList(s []string) []string {
	if len(s) > 0 {
		return s
	}
	return []string{
		"stun:stun.l.google.com:19302",
		"stun:stun1.l.google.com:19302",
	}
}

// buildICEServers orders STUN then TURN; expands turn URLs.
func buildICEServers(cfg Config) []webrtc.ICEServer {
	var ice []webrtc.ICEServer
	for _, u := range defaultSTUNList(cfg.STUN) {
		ice = append(ice, webrtc.ICEServer{URLs: []string{normalizeICEURL(u, "stun")}})
	}
	if len(cfg.TURNURLs) > 0 {
		urls := make([]string, 0, len(cfg.TURNURLs)*2)
		for _, u := range cfg.TURNURLs {
			u = normalizeICEURL(u, "turn")
			urls = append(urls, u)
			// also try turns: if plain turn given (TLS TURN when available)
			if strings.HasPrefix(u, "turn:") && !strings.Contains(u, "turns:") {
				// keep only explicit turns from operator — do not invent TLS
			}
		}
		ice = append(ice, webrtc.ICEServer{
			URLs:       urls,
			Username:   cfg.TURNUser,
			Credential: cfg.TURNPass,
		})
	}
	return ice
}

func normalizeICEURL(u, kind string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return u
	}
	if strings.Contains(u, ":") && !strings.Contains(u, "://") &&
		!strings.HasPrefix(u, "stun:") && !strings.HasPrefix(u, "stuns:") &&
		!strings.HasPrefix(u, "turn:") && !strings.HasPrefix(u, "turns:") {
		return kind + ":" + u
	}
	return u
}

func iceTransportPolicy(cfg Config) webrtc.ICETransportPolicy {
	if cfg.Policy == ICEPolicyRelay && len(cfg.TURNURLs) > 0 {
		return webrtc.ICETransportPolicyRelay
	}
	return webrtc.ICETransportPolicyAll
}
