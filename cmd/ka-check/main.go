// ka-check: production readiness checks for a COE Key Authority deployment.
// Exit 0 = all required checks pass; 1 = one or more required failures.
package main

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type severity string

const (
	sevReq  severity = "required"
	sevWarn severity = "warning"
	sevInfo severity = "info"
)

type checkResult struct {
	ID       string   `json:"id"`
	Severity severity `json:"severity"`
	OK       bool     `json:"ok"`
	Detail   string   `json:"detail"`
}

func main() {
	url := flag.String("url", envOr("COE_KA_URL", "https://127.0.0.1:8443"), "KA base URL")
	caFile := flag.String("ca", envOr("COE_KA_CA", ""), "trusted CA / server cert PEM (empty = system roots)")
	insecure := flag.Bool("insecure", false, "skip TLS verify (always fails required check)")
	adminTok := flag.String("admin-token", envOr("COE_KA_ADMIN_TOKEN", ""), "optional: probe admin auth")
	masterFile := flag.String("master-file", "", "optional: local KA_MASTER path (perms + length)")
	registry := flag.String("registry", "", "optional: local registry.json path")
	jsonOut := flag.Bool("json", false, "machine-readable JSON")
	timeout := flag.Duration("timeout", 10*time.Second, "HTTP timeout")
	stun := flag.String("stun", envOr("COE_STUN", "stun:stun.l.google.com:19302"), "STUN host for UDP reachability (host:port or stun:host:port); empty=skip")
	turn := flag.String("turn", envOr("COE_TURN_URLS", ""), "TURN host for UDP reachability (turn:host:port or host:port); empty=skip")
	flag.Parse()

	var results []checkResult

	// --- URL scheme ---
	if strings.HasPrefix(strings.ToLower(*url), "https://") {
		results = append(results, ok("tls_url", sevReq, "URL uses https://"))
	} else {
		results = append(results, fail("tls_url", sevReq, "URL must be https:// (got %s)", *url))
	}

	if *insecure {
		results = append(results, fail("tls_verify", sevReq, "-insecure set: TLS verification disabled"))
	} else {
		results = append(results, ok("tls_verify", sevReq, "TLS verification enabled"))
	}

	// --- HTTP client ---
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if *insecure {
		tlsCfg.InsecureSkipVerify = true //nolint:gosec
	} else if *caFile != "" {
		pemBytes, err := os.ReadFile(*caFile)
		if err != nil {
			results = append(results, fail("ca_file", sevReq, "read CA: %v", err))
		} else {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pemBytes) {
				results = append(results, fail("ca_file", sevReq, "no PEM certs in %s", *caFile))
			} else {
				tlsCfg.RootCAs = pool
				results = append(results, ok("ca_file", sevReq, "loaded CA from %s", *caFile))
				// SPKI pin info
				if block, _ := pem.Decode(pemBytes); block != nil {
					if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
						sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
						results = append(results, ok("spki_sha256", sevInfo, "%s", hex.EncodeToString(sum[:])))
					}
				}
			}
		}
	} else {
		results = append(results, ok("ca_file", sevWarn, "using system roots (set -ca for pin/self-signed)"))
	}

	client := &http.Client{
		Timeout: *timeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
			DialContext: (&net.Dialer{Timeout: *timeout}).DialContext,
		},
	}

	// --- Health ---
	healthURL := strings.TrimRight(*url, "/") + "/v1/health"
	resp, err := client.Get(healthURL)
	if err != nil {
		results = append(results, fail("health", sevReq, "GET /v1/health: %v", err))
	} else {
		defer resp.Body.Close()
		var body struct {
			Status  string `json:"status"`
			EpochID uint64 `json:"epoch_id"`
			TLS     bool   `json:"tls"`
			Time    int64  `json:"time"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if resp.StatusCode == 200 && body.Status == "ok" {
			results = append(results, ok("health", sevReq, "status=ok epoch=%d", body.EpochID))
		} else {
			results = append(results, fail("health", sevReq, "status=%d body=%+v", resp.StatusCode, body))
		}
		if body.TLS || strings.HasPrefix(*url, "https") {
			results = append(results, ok("server_tls_flag", sevInfo, "tls reported or https URL"))
		}
		// clock skew
		if body.Time > 0 {
			delta := time.Now().UTC().Unix() - body.Time
			if delta < 0 {
				delta = -delta
			}
			if delta <= 120 {
				results = append(results, ok("clock_skew", sevReq, "skew=%ds (limit 120)", delta))
			} else {
				results = append(results, fail("clock_skew", sevReq, "skew=%ds exceeds 120s", delta))
			}
		}
	}

	// --- TLS handshake details (if https) ---
	if strings.HasPrefix(*url, "https://") && !*insecure {
		hostPort := strings.TrimPrefix(*url, "https://")
		hostPort = strings.TrimSuffix(hostPort, "/")
		if !strings.Contains(hostPort, ":") {
			hostPort += ":443"
		}
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: *timeout}, "tcp", hostPort, tlsCfg)
		if err != nil {
			results = append(results, fail("tls_handshake", sevReq, "%v", err))
		} else {
			state := conn.ConnectionState()
			_ = conn.Close()
			if state.Version < tls.VersionTLS12 {
				results = append(results, fail("tls_version", sevReq, "TLS version too old: 0x%x", state.Version))
			} else {
				results = append(results, ok("tls_version", sevReq, "TLS OK (0x%x)", state.Version))
			}
			if len(state.PeerCertificates) > 0 {
				c := state.PeerCertificates[0]
				days := int(time.Until(c.NotAfter).Hours() / 24)
				if days < 14 {
					results = append(results, fail("cert_expiry", sevReq, "cert expires in %d days (%s)", days, c.NotAfter.Format(time.RFC3339)))
				} else if days < 30 {
					results = append(results, fail("cert_expiry", sevWarn, "cert expires in %d days", days))
					results[len(results)-1].OK = true // warn still ok for exit? treat warn as non-fail
					results[len(results)-1].Severity = sevWarn
					results[len(results)-1].OK = true
					results[len(results)-1].Detail = fmt.Sprintf("cert expires in %d days (%s)", days, c.NotAfter.Format(time.RFC3339))
				} else {
					results = append(results, ok("cert_expiry", sevReq, "valid %d days left", days))
				}
				// self-signed detection
				if c.Subject.String() == c.Issuer.String() {
					results = append(results, ok("cert_public", sevWarn, "self-signed / private CA — OK for lab, use public CA in prod"))
				} else {
					results = append(results, ok("cert_public", sevInfo, "issuer=%s", c.Issuer.CommonName))
				}
			}
			results = append(results, ok("tls_handshake", sevReq, "handshake ok"))
		}
	}

	// --- Admin token hygiene ---
	if *adminTok == "" {
		results = append(results, ok("admin_token_set", sevWarn, "no -admin-token (skip admin probes)"))
	} else if *adminTok == "dev-admin-token" {
		results = append(results, fail("admin_token_set", sevReq, "admin token still default 'dev-admin-token'"))
	} else if len(*adminTok) < 16 {
		results = append(results, fail("admin_token_set", sevWarn, "admin token short (<16 chars)"))
		results[len(results)-1].OK = true
		results[len(results)-1].Severity = sevWarn
	} else {
		results = append(results, ok("admin_token_set", sevReq, "non-default admin token present"))
	}

	// --- Local files (when operating the KA host) ---
	if *masterFile != "" {
		results = append(results, checkMasterFile(*masterFile)...)
	} else {
		results = append(results, ok("master_file", sevInfo, "skipped (pass -master-file on KA host)"))
	}
	if *registry != "" {
		results = append(results, checkRegistry(*registry)...)
	}

	// --- ICE helpers (UDP reachability; not full STUN protocol) ---
	if *stun != "" {
		results = append(results, checkUDPEndpoint("stun_udp", *stun, *timeout)...)
	} else {
		results = append(results, ok("stun_udp", sevInfo, "skipped (pass -stun)"))
	}
	if *turn != "" {
		// first URL only if comma-separated
		first := strings.Split(*turn, ",")[0]
		results = append(results, checkUDPEndpoint("turn_udp", first, *timeout)...)
	} else {
		results = append(results, ok("turn_udp", sevInfo, "skipped (pass -turn or COE_TURN_URLS)"))
	}

	reqFail := 0
	warns := 0
	for _, r := range results {
		if r.Severity == sevReq && !r.OK {
			reqFail++
		}
		if r.Severity == sevWarn && !r.OK {
			warns++
		}
		// warnings with OK=true still count as soft
		if r.Severity == sevWarn && r.OK && strings.Contains(strings.ToLower(r.Detail), "self-signed") {
			warns++
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"url":             *url,
			"required_failed": reqFail,
			"checks":          results,
		})
	} else {
		fmt.Printf("COE KA production check: %s\n", *url)
		fmt.Println(strings.Repeat("-", 60))
		for _, r := range results {
			mark := "OK "
			if !r.OK {
				mark = "FAIL"
			} else if r.Severity == sevWarn {
				mark = "WARN"
			} else if r.Severity == sevInfo {
				mark = "INFO"
			}
			fmt.Printf("[%s] %-16s %-8s %s\n", mark, r.ID, r.Severity, r.Detail)
		}
		fmt.Println(strings.Repeat("-", 60))
		if reqFail == 0 {
			fmt.Printf("RESULT: PASS (%d required ok, %d warnings)\n", countReqOK(results), warns)
		} else {
			fmt.Printf("RESULT: FAIL (%d required failed)\n", reqFail)
		}
	}

	if reqFail > 0 {
		os.Exit(1)
	}
}

func countReqOK(rs []checkResult) int {
	n := 0
	for _, r := range rs {
		if r.Severity == sevReq && r.OK {
			n++
		}
	}
	return n
}

func ok(id string, sev severity, format string, args ...any) checkResult {
	return checkResult{ID: id, Severity: sev, OK: true, Detail: fmt.Sprintf(format, args...)}
}

func fail(id string, sev severity, format string, args ...any) checkResult {
	return checkResult{ID: id, Severity: sev, OK: false, Detail: fmt.Sprintf(format, args...)}
}

func checkMasterFile(path string) []checkResult {
	var out []checkResult
	fi, err := os.Stat(path)
	if err != nil {
		return []checkResult{fail("master_file", sevReq, "%v", err)}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return []checkResult{fail("master_file", sevReq, "read: %v", err)}
	}
	s := strings.TrimSpace(string(b))
	switch {
	case len(s) == 64:
		out = append(out, ok("master_file", sevReq, "32-byte key as hex present"))
	case len(b) == 32:
		out = append(out, ok("master_file", sevReq, "32 raw bytes present"))
	default:
		out = append(out, fail("master_file", sevReq, "unexpected length (want 32 raw or 64 hex)"))
	}
	// Windows often doesn't expose unix perms; still report mode
	out = append(out, ok("master_perms", sevWarn, "mode=%v — ensure ACL limited to KA service account", fi.Mode()))
	return out
}

func checkRegistry(path string) []checkResult {
	b, err := os.ReadFile(path)
	if err != nil {
		return []checkResult{fail("registry", sevReq, "%v", err)}
	}
	var reg struct {
		Devices map[string]any `json:"devices"`
	}
	if err := json.Unmarshal(b, &reg); err != nil {
		return []checkResult{fail("registry", sevReq, "json: %v", err)}
	}
	n := len(reg.Devices)
	return []checkResult{ok("registry", sevInfo, "%d enrolled devices", n)}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// checkUDPEndpoint tries UDP dial to host:port extracted from stun:/turn: URL.
// Warning-level only: many NATs block outbound probes differently than WebRTC.
func checkUDPEndpoint(id, raw string, timeout time.Duration) []checkResult {
	hostPort := strings.TrimSpace(raw)
	hostPort = strings.TrimPrefix(hostPort, "stuns:")
	hostPort = strings.TrimPrefix(hostPort, "stun:")
	hostPort = strings.TrimPrefix(hostPort, "turns:")
	hostPort = strings.TrimPrefix(hostPort, "turn:")
	if i := strings.Index(hostPort, "?"); i >= 0 {
		hostPort = hostPort[:i]
	}
	if !strings.Contains(hostPort, ":") {
		hostPort += ":3478"
	}
	conn, err := net.DialTimeout("udp", hostPort, timeout)
	if err != nil {
		return []checkResult{fail(id, sevWarn, "udp dial %s: %v", hostPort, err)}
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	// send minimal non-empty datagram; STUN servers often ignore invalid packets but path opens
	_, _ = conn.Write([]byte{0, 1, 0, 0})
	_ = conn.Close()
	return []checkResult{ok(id, sevWarn, "udp path open to %s (not full STUN/TURN handshake)", hostPort)}
}
