// Copyright COE contributors. AGPL-3.0-or-later — server-side admin tool
package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/telecon/coe/internal/kaapi"
	"github.com/telecon/coe/internal/tlsutil"
)

func main() {
	kaURL := flag.String("ka", envOr("COE_KA_URL", "https://127.0.0.1:8443"), "KA URL")
	kaCA := flag.String("ka-ca", envOr("COE_KA_CA", "data/ka/tls/server.crt"), "KA CA PEM")
	insecure := flag.Bool("ka-insecure", false, "skip TLS verify")
	adminTok := flag.String("admin-token", envOr("COE_KA_ADMIN_TOKEN", "dev-admin-token"), "admin token")
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}

	tlsCfg, err := tlsutil.ClientTLS(*kaCA, *insecure)
	if err != nil {
		// allow empty CA with insecure
		if *insecure {
			tlsCfg = &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12} //nolint:gosec
		} else if *kaCA == "" {
			tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
		} else {
			log.Fatal(err)
		}
	}
	c, err := kaapi.NewClientOpts(*kaURL, "", kaapi.ClientOptions{TLS: tlsCfg})
	if err != nil {
		log.Fatal(err)
	}
	c.AdminToken = *adminTok

	switch args[0] {
	case "voucher", "voucher-create":
		maxUses := 1
		ttl := 168
		label := ""
		org := ""
		// optional flags after subcommand via env or simple parse
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "-max-uses":
				i++
				if i < len(args) {
					fmt.Sscanf(args[i], "%d", &maxUses)
				}
			case "-ttl-hours":
				i++
				if i < len(args) {
					fmt.Sscanf(args[i], "%d", &ttl)
				}
			case "-label":
				i++
				if i < len(args) {
					label = args[i]
				}
			case "-org":
				i++
				if i < len(args) {
					org = args[i]
				}
			}
		}
		resp, err := c.CreateVoucher(kaapi.CreateVoucherRequest{
			MaxUses: maxUses, TTLHours: ttl, Label: label, OrgID: org, Profile: "strong",
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("voucher_id=%s\n", resp.ID)
		fmt.Printf("code=%s\n", resp.Code)
		fmt.Printf("max_uses=%d expires_at=%d\n", resp.MaxUses, resp.ExpiresAt)
		fmt.Println("Share `code` with the device owner once. It is not stored in plaintext on KA.")
	case "voucher-list":
		list, err := c.ListVouchers()
		if err != nil {
			log.Fatal(err)
		}
		b, _ := json.MarshalIndent(list, "", "  ")
		fmt.Println(string(b))
	case "revoke":
		if len(args) < 2 {
			log.Fatal("usage: coe-admin revoke <device_id>")
		}
		if err := c.Revoke(args[1], "admin"); err != nil {
			log.Fatal(err)
		}
		fmt.Println("revoked", args[1])
	case "health":
		h, err := c.Health()
		if err != nil {
			log.Fatal(err)
		}
		b, _ := json.MarshalIndent(h, "", "  ")
		fmt.Println(string(b))
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `coe-admin — KA admin CLI (AGPL)

  coe-admin [-ka URL] [-ka-ca PEM] [-admin-token TOK] <cmd>

Commands:
  voucher [-max-uses N] [-ttl-hours H] [-label L] [-org O]
  voucher-list
  revoke <device_id>
  health
`)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
