package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/telecon/coe/internal/agent"
	"github.com/telecon/coe/internal/config"
	"github.com/telecon/coe/internal/device"
	"github.com/telecon/coe/internal/kaapi"
	"github.com/telecon/coe/internal/p2p"
	"github.com/telecon/coe/internal/tlsutil"
)

// Set via -ldflags "-X main.version=..."
var version = "0.1.0-beta"

func main() {
	cfgPath := flag.String("config", "", "node config JSON")
	deviceID := flag.String("device-id", "", "device id")
	identityPath := flag.String("identity", "", "identity.json path")
	storeDir := flag.String("store", "", "key store dir")
	kaURL := flag.String("ka", "https://127.0.0.1:8443", "KA base URL")
	kaCA := flag.String("ka-ca", "data/ka/tls/server.crt", "KA CA/server cert to trust")
	kaInsecure := flag.Bool("ka-insecure", false, "skip KA TLS verify (dev only)")
	listen := flag.String("listen", "", "P2P listen host:port")
	apiAddr := flag.String("api", "", "localhost app API (e.g. 127.0.0.1:7701)")
	apiToken := flag.String("api-token", "", "Bearer token for local API")
	peer := flag.String("peer", "", "one-shot: device_id@host:port")
	adminTok := flag.String("admin-token", "", "admin token for enroll (prefer -voucher)")
	voucher := flag.String("voucher", "", "one-time enroll voucher code")
	enroll := flag.Bool("enroll", false, "enroll with KA then exit")
	message := flag.String("msg", "hello from COE", "one-shot message")
	service := flag.Bool("service", false, "run as Windows service (Windows only)")
	// ICE / TURN for WebRTC (comma-separated STUN URLs; TURN via env COE_TURN_*)
	stun := flag.String("stun", "", "comma-separated STUN URLs (default public STUN)")
	flag.Parse()

	cfg := loadConfig(*cfgPath, *deviceID, *identityPath, *storeDir, *kaURL, *listen, *apiAddr, *apiToken, *adminTok)
	if *kaCA != "" {
		cfg.KACAFile = *kaCA
	}
	if *kaInsecure {
		cfg.KAInsecure = true
	}
	if *voucher != "" {
		cfg.VoucherCode = *voucher
	}
	if *stun != "" {
		cfg.STUN = splitCSV(*stun)
	}
	loadICEFromEnv(&cfg)

	run := func() {
		runAgent(cfg, *enroll, *peer, *message)
	}

	if *service {
		if err := runWindowsService(cfg.DeviceID, run); err != nil {
			log.Fatalf("service: %v", err)
		}
		return
	}
	run()
}

func runAgent(cfg config.NodeConfig, enroll bool, peer, message string) {
	id, err := device.LoadIdentity(cfg.IdentityPath)
	if err != nil {
		log.Fatalf("identity: %v", err)
	}
	if id.DeviceID != cfg.DeviceID {
		log.Fatal("identity device_id mismatch")
	}
	cfg.Profile = id.Profile
	if cfg.Profile == "" {
		cfg.Profile = "strong"
	}

	client, err := newKAClient(cfg)
	if err != nil {
		log.Fatalf("ka client: %v", err)
	}
	if len(id.SignPrivate) > 0 {
		sk, err := id.SignPrivateKey()
		if err != nil {
			log.Fatalf("sign key: %v", err)
		}
		client.SetSignKey(sk)
	}

	if enroll {
		if cfg.VoucherCode == "" && cfg.AdminToken == "" {
			log.Fatal("enroll needs -voucher CODE or -admin-token (voucher preferred)")
		}
		// Prefer voucher: clear admin so client uses voucher path
		if cfg.VoucherCode != "" {
			client.AdminToken = ""
		}
		er, err := client.Enroll(kaapi.EnrollRequest{
			DeviceID:          id.DeviceID,
			IdentityPK:        id.PublicKey,
			SignPK:            id.SignPublic,
			OrgID:             id.OrgID,
			EnrollmentCounter: id.EnrollmentCounter,
			Profile:           cfg.Profile,
			VoucherCode:       cfg.VoucherCode,
		})
		if err != nil {
			log.Fatalf("enroll: %v", err)
		}
		_ = device.SaveIdentity(cfg.IdentityPath, id)
		fmt.Printf("enrolled %s counter=%d profile=%s\n", er.DeviceID, er.EnrollmentCounter, er.Profile)
		return
	}

	store, err := device.NewFileStore(cfg.StoreDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	iss, err := issueKey(client, cfg)
	if err != nil {
		log.Fatalf("key issue: %v", err)
	}
	meta := device.Meta{
		EpochID: iss.EpochID, KeySerial: iss.KeySerial,
		NotBefore: iss.NotBefore, NotAfter: iss.NotAfter,
		GraceSec: int64(iss.GraceSeconds), Profile: iss.Profile, DeviceID: iss.DeviceID,
	}
	if err := store.PutGeneral(iss.EpochID, iss.GeneralKey, meta); err != nil {
		log.Fatalf("store key: %v", err)
	}
	if iss.EpochID >= 2 {
		_ = store.WipeBefore(iss.EpochID - 1)
	}
	log.Printf("general key epoch=%d serial=%d profile=%s protect=%v",
		iss.EpochID, iss.KeySerial, iss.Profile, device.ProtectAvailable())

	creds := p2p.LocalCreds{
		DeviceID: cfg.DeviceID, IdentitySK: id.PrivateKey, IdentityPK: id.PublicKey,
		GeneralKey: iss.GeneralKey, KeySerial: iss.KeySerial, EpochID: iss.EpochID, Profile: iss.Profile,
	}
	hub := agent.NewHub(creds)
	hub.LoadPeers(cfg.Peers)
	hub.SetICE(agent.ICEConfig{
		STUN: cfg.STUN, TURNURLs: cfg.TURNURLs, TURNUser: cfg.TURNUser, TURNPass: cfg.TURNPass,
	})

	if peer != "" {
		peerID, addr, err := parsePeer(peer)
		if err != nil {
			log.Fatal(err)
		}
		if err := hub.Connect(peerID, addr); err != nil {
			log.Fatalf("connect: %v", err)
		}
		if err := hub.Send(peerID, []byte(message)); err != nil {
			log.Fatalf("send: %v", err)
		}
		fmt.Printf("sent: %s\n", message)
		if m, ok := hub.WaitInbox(5 * time.Second); ok {
			fmt.Printf("recv: %s\n", m.Body)
		}
		hub.CloseAll()
		return
	}

	if cfg.ListenAddr == "" && cfg.APIAddr == "" {
		log.Fatal("agent mode needs -listen and/or -api (or -peer for one-shot)")
	}

	if cfg.ListenAddr != "" {
		go serveP2P(cfg.ListenAddr, hub)
	}

	epochMeta := iss
	if cfg.APIAddr != "" {
		api := &agent.LocalAPI{
			Hub:      hub,
			DeviceID: cfg.DeviceID,
			Token:    cfg.APIToken,
			Status: func() map[string]any {
				return map[string]any{
					"version":    version,
					"epoch_id":   epochMeta.EpochID,
					"key_serial": epochMeta.KeySerial,
					"profile":    epochMeta.Profile,
					"ka_url":     cfg.KAURL,
					"listen":     cfg.ListenAddr,
					"protect":    device.ProtectAvailable(),
				}
			},
		}
		go func() {
			log.Printf("local app API on %s", cfg.APIAddr)
			if err := api.ListenAndServe(cfg.APIAddr); err != nil {
				log.Fatalf("api: %v", err)
			}
		}()
	}

	// update check (log only in beta; no auto-download)
	go func() {
		check := func() {
			u, err := client.CheckUpdate(version, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				log.Printf("update check: %v", err)
				return
			}
			if u.UpdateAvailable {
				log.Printf("update available: current=%s latest=%s forced=%v url=%s notes=%s",
					u.Current, u.Latest, u.Forced, u.DownloadURL, u.Notes)
			}
		}
		check()
		t := time.NewTicker(12 * time.Hour)
		defer t.Stop()
		for range t.C {
			check()
		}
	}()

	go func() {
		t := time.NewTicker(1 * time.Hour)
		defer t.Stop()
		for range t.C {
			newIss, err := issueKey(client, cfg)
			if err != nil {
				log.Printf("key refresh failed: %v", err)
				continue
			}
			if newIss.EpochID != epochMeta.EpochID || newIss.KeySerial != epochMeta.KeySerial {
				epochMeta = newIss
				_ = store.PutGeneral(newIss.EpochID, newIss.GeneralKey, device.Meta{
					EpochID: newIss.EpochID, KeySerial: newIss.KeySerial,
					NotBefore: newIss.NotBefore, NotAfter: newIss.NotAfter,
					GraceSec: int64(newIss.GraceSeconds), Profile: newIss.Profile, DeviceID: newIss.DeviceID,
				})
				hub.SetCreds(p2p.LocalCreds{
					DeviceID: cfg.DeviceID, IdentitySK: id.PrivateKey, IdentityPK: id.PublicKey,
					GeneralKey: newIss.GeneralKey, KeySerial: newIss.KeySerial,
					EpochID: newIss.EpochID, Profile: newIss.Profile,
				})
				log.Printf("refreshed general key epoch=%d", newIss.EpochID)
			}
		}
	}()

	log.Printf("coe-node agent device=%s p2p=%s api=%s ka=%s", cfg.DeviceID, cfg.ListenAddr, cfg.APIAddr, cfg.KAURL)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	log.Printf("shutting down")
	hub.CloseAll()
}

func newKAClient(cfg config.NodeConfig) (*kaapi.Client, error) {
	var tlsCfg *tls.Config
	var err error
	if cfg.KAInsecure {
		tlsCfg, err = tlsutil.ClientTLS("", true)
	} else {
		tlsCfg, err = tlsutil.ClientTLS(cfg.KACAFile, false)
	}
	if err != nil {
		return nil, err
	}
	c, err := kaapi.NewClientOpts(cfg.KAURL, cfg.DeviceID, kaapi.ClientOptions{TLS: tlsCfg})
	if err != nil {
		return nil, err
	}
	c.AdminToken = cfg.AdminToken
	return c, nil
}

func serveP2P(addr string, hub *agent.Hub) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("p2p listen: %v", err)
	}
	log.Printf("P2P listen %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go func(c net.Conn) {
			if err := hub.AcceptConn(c); err != nil {
				log.Printf("handshake: %v", err)
			}
		}(conn)
	}
}

func issueKey(client *kaapi.Client, cfg config.NodeConfig) (kaapi.IssueResponse, error) {
	// optional pre-sync from health for large skew
	if h, err := client.Health(); err == nil && h.Time > 0 {
		device.ApplyKATime(h.Time)
	}
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	resp, err := client.IssueKey(kaapi.IssueRequest{
		DeviceID:   cfg.DeviceID,
		EpochHint:  device.CurrentEpoch(),
		Nonce:      nonce,
		ClientTime: device.CorrectedNow().UTC().Unix(),
		Profile:    cfg.Profile,
	})
	if err != nil {
		return resp, err
	}
	if resp.KATime > 0 {
		device.ApplyKATime(resp.KATime)
	}
	return resp, nil
}

func loadConfig(cfgPath, deviceID, identity, store, ka, listen, api, apiTok, admin string) config.NodeConfig {
	var cfg config.NodeConfig
	if cfgPath != "" {
		var err error
		cfg, err = config.LoadNode(cfgPath)
		if err != nil {
			log.Fatalf("config: %v", err)
		}
	}
	if deviceID != "" {
		cfg.DeviceID = deviceID
	}
	if identity != "" {
		cfg.IdentityPath = identity
	}
	if store != "" {
		cfg.StoreDir = store
	}
	if ka != "" {
		cfg.KAURL = ka
	}
	if listen != "" {
		cfg.ListenAddr = listen
	}
	if api != "" {
		cfg.APIAddr = api
	}
	if apiTok != "" {
		cfg.APIToken = apiTok
	}
	if admin != "" {
		cfg.AdminToken = admin
	}
	if cfg.KAURL == "" {
		cfg.KAURL = "https://127.0.0.1:8443"
	}
	if cfg.KACAFile == "" {
		cfg.KACAFile = "data/ka/tls/server.crt"
	}
	if cfg.DeviceID == "" {
		log.Fatal("device-id required")
	}
	if cfg.IdentityPath == "" {
		cfg.IdentityPath = fmt.Sprintf("data/devices/%s/identity.json", cfg.DeviceID)
	}
	if cfg.StoreDir == "" {
		cfg.StoreDir = fmt.Sprintf("data/devices/%s/keys", cfg.DeviceID)
	}
	return cfg
}

func parsePeer(s string) (deviceID, addr string, err error) {
	for i := 0; i < len(s); i++ {
		if s[i] == '@' {
			return s[:i], s[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("peer must be device_id@host:port")
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

func loadICEFromEnv(cfg *config.NodeConfig) {
	if v := os.Getenv("COE_STUN"); v != "" && len(cfg.STUN) == 0 {
		cfg.STUN = splitCSV(v)
	}
	if v := os.Getenv("COE_TURN_URLS"); v != "" && len(cfg.TURNURLs) == 0 {
		cfg.TURNURLs = splitCSV(v)
	}
	if v := os.Getenv("COE_TURN_USER"); v != "" && cfg.TURNUser == "" {
		cfg.TURNUser = v
	}
	if v := os.Getenv("COE_TURN_PASS"); v != "" && cfg.TURNPass == "" {
		cfg.TURNPass = v
	}
	if v := os.Getenv("COE_SIGNAL_URL"); v != "" && cfg.SignalURL == "" {
		cfg.SignalURL = v
	}
}

// silence unused if no sign
var _ ed25519.PrivateKey
