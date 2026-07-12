// Copyright COE contributors. AGPL-3.0-or-later — see LICENSES/AGPL-3.0.txt
package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/telecon/coe/internal/signal"
	"github.com/telecon/coe/internal/tlsutil"
)

func main() {
	listen := flag.String("listen", envOr("COE_SIGNAL_LISTEN", "127.0.0.1:8450"), "listen address")
	tlsCert := flag.String("tls-cert", os.Getenv("COE_SIGNAL_TLS_CERT"), "optional TLS cert")
	tlsKey := flag.String("tls-key", os.Getenv("COE_SIGNAL_TLS_KEY"), "optional TLS key")
	flag.Parse()

	srv := &signal.Server{Hub: signal.NewHub()}
	handler := withCORS(srv.Handler())
	httpSrv := &http.Server{
		Addr:              *listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if *tlsCert != "" && *tlsKey != "" {
		cert, err := tlsutil.EnsureServerCert(*tlsCert, *tlsKey, []string{"localhost", "127.0.0.1"})
		if err != nil {
			// try load only
			log.Fatalf("tls: %v", err)
		}
		httpSrv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
		log.Printf("COE signaling TLS on %s (SDP/ICE only)", *listen)
		log.Fatal(httpSrv.ListenAndServeTLS("", ""))
	}
	log.Printf("COE signaling on http://%s (SDP/ICE only — put TLS reverse proxy in prod)", *listen)
	log.Fatal(httpSrv.ListenAndServe())
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
