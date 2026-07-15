// Copyright COE contributors. AGPL-3.0-or-later — see LICENSES/AGPL-3.0.txt
package main

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	coecrypto "github.com/telecon/coe/internal/crypto"
	"github.com/telecon/coe/internal/kaapi"
	"github.com/telecon/coe/internal/tlsutil"
)

func main() {
	listen := flag.String("listen", envOr("COE_KA_LISTEN", "127.0.0.1:8443"), "listen address")
	masterFile := flag.String("master", envOr("COE_KA_MASTER_FILE", "data/ka/master.key"), "KA_MASTER file")
	registry := flag.String("registry", envOr("COE_KA_REGISTRY", "data/ka/registry.json"), "JSON registry path (ignored if -sqlite set)")
	sqlitePath := flag.String("sqlite", envOr("COE_KA_SQLITE", ""), "SQLite registry path (e.g. data/ka/registry.db); preferred for multi-device")
	adminTok := flag.String("admin-token", envOr("COE_KA_ADMIN_TOKEN", "dev-admin-token"), "admin bearer token")
	tlsCert := flag.String("tls-cert", envOr("COE_KA_TLS_CERT", "data/ka/tls/server.crt"), "TLS cert PEM")
	tlsKey := flag.String("tls-key", envOr("COE_KA_TLS_KEY", "data/ka/tls/server.key"), "TLS key PEM")
	plain := flag.Bool("http", false, "plain HTTP (insecure demo)")
	noSig := flag.Bool("no-require-sig", false, "allow unsigned issue (legacy only)")
	rateLimit := flag.Int("rate-limit", 60, "max enroll/issue requests per IP per minute (0=off)")
	prod := flag.Bool("prod", false, "production mode: require non-default admin token, rate limit on")
	auditFile := flag.String("audit-file", envOr("COE_KA_AUDIT_FILE", ""), "append JSON audit lines to file (empty=stdout only)")
	flag.Parse()

	if *prod {
		if *adminTok == "" || *adminTok == "dev-admin-token" {
			log.Fatal("prod mode: set COE_KA_ADMIN_TOKEN to a strong non-default value")
		}
		if *plain {
			log.Fatal("prod mode: plain HTTP not allowed")
		}
	}

	master, err := loadOrCreateMaster(*masterFile)
	if err != nil {
		log.Fatalf("master: %v", err)
	}
	var store kaapi.Store
	if *sqlitePath != "" {
		if err := os.MkdirAll(filepath.Dir(*sqlitePath), 0o700); err != nil {
			log.Fatalf("sqlite dir: %v", err)
		}
		sqlStore, err := kaapi.OpenSQLStore(*sqlitePath)
		if err != nil {
			log.Fatalf("sqlite: %v", err)
		}
		store = sqlStore
		log.Printf("registry backend sqlite %s", *sqlitePath)
	} else {
		reg, err := kaapi.LoadRegistry(*registry)
		if err != nil {
			log.Fatalf("registry: %v", err)
		}
		store = reg
		log.Printf("registry backend json %s", *registry)
	}
	xo, err := coecrypto.NewXoroshiroFromMaster(master)
	if err != nil {
		log.Fatalf("xoroshiro: %v", err)
	}

	var lim *kaapi.RateLimiter
	if *rateLimit > 0 {
		lim = kaapi.NewRateLimiter(*rateLimit, time.Minute)
	}

	auditW := io.Writer(os.Stdout)
	if *auditFile != "" {
		if err := os.MkdirAll(filepath.Dir(*auditFile), 0o700); err != nil {
			log.Fatalf("audit dir: %v", err)
		}
		f, err := os.OpenFile(*auditFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			log.Fatalf("audit file: %v", err)
		}
		auditW = io.MultiWriter(os.Stdout, f)
		log.Printf("audit log file %s", *auditFile)
	}
	srv := &kaapi.Server{
		Master:            master,
		Registry:          store,
		XO:                xo,
		Audit:             kaapi.NewAuditor(auditW),
		AdminTok:          *adminTok,
		RequireSignature:  !*noSig,
		RequireAdminToken: *prod,
		Limit:             lim,
	}

	handler := srv.HandlerWithLimit()
	addr := *listen

	if *plain {
		srv.TLSActive = false
		log.Printf("COE KA plain HTTP on %s (INSECURE)", addr)
		log.Fatal(http.ListenAndServe(addr, handler))
	}

	hosts := []string{"localhost", "127.0.0.1"}
	if h := os.Getenv("COE_KA_TLS_HOSTS"); h != "" {
		hosts = strings.Split(h, ",")
	}
	cert, err := tlsutil.EnsureServerCert(*tlsCert, *tlsKey, hosts)
	if err != nil {
		log.Fatalf("tls cert: %v", err)
	}
	srv.TLSActive = true
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("COE Key Authority TLS on %s cert=%s rate_limit=%d prod=%v", addr, *tlsCert, *rateLimit, *prod)
	log.Fatal(httpSrv.ListenAndServeTLS("", ""))
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func loadOrCreateMaster(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		s := strings.TrimSpace(string(b))
		if len(s) == 64 {
			return hex.DecodeString(s)
		}
		if len(b) == 32 {
			return b, nil
		}
		raw := []byte(s)
		if len(raw) == 32 {
			return raw, nil
		}
		return nil, fmt.Errorf("master file must be 32 raw bytes or 64 hex chars")
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)+"\n"), 0o600); err != nil {
		return nil, err
	}
	log.Printf("generated new KA_MASTER at %s", path)
	return key, nil
}
