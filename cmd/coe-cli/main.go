package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	api := flag.String("api", envOr("COE_API", "http://127.0.0.1:7701"), "local agent API base")
	token := flag.String("token", envOr("COE_API_TOKEN", ""), "Bearer token")
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}
	c := &client{base: strings.TrimRight(*api, "/"), token: *token}

	switch args[0] {
	case "health":
		printJSON(c.get("/v1/health"))
	case "status":
		printJSON(c.get("/v1/status"))
	case "peers":
		if len(args) >= 2 && args[1] == "add" {
			if len(args) < 4 {
				fatal("usage: coe-cli peers add <device_id> <host:port>")
			}
			printJSON(c.post("/v1/peers", map[string]string{"device_id": args[2], "addr": args[3]}))
			return
		}
		printJSON(c.get("/v1/peers"))
	case "connect":
		if len(args) < 2 {
			fatal("usage: coe-cli connect <device_id> [host:port]")
		}
		body := map[string]string{"device_id": args[1]}
		if len(args) >= 3 {
			body["addr"] = args[2]
		}
		printJSON(c.post("/v1/sessions", body))
	case "webrtc":
		// coe-cli webrtc dial|accept <device_id> <signal_url>
		if len(args) < 4 {
			fatal("usage: coe-cli webrtc dial|accept <device_id> <signal_url>")
		}
		printJSON(c.post("/v1/sessions/webrtc", map[string]any{
			"role":       args[1],
			"device_id":  args[2],
			"signal_url": args[3],
			"wait_sec":   60,
		}))
	case "sessions":
		printJSON(c.get("/v1/sessions"))
	case "send":
		if len(args) < 3 {
			fatal("usage: coe-cli send <device_id> <text...>")
		}
		text := strings.Join(args[2:], " ")
		printJSON(c.post("/v1/send", map[string]string{"to": args[1], "text": text}))
	case "inbox":
		wait := ""
		clear := true
		for _, a := range args[1:] {
			if strings.HasPrefix(a, "--wait=") {
				wait = strings.TrimPrefix(a, "--wait=")
			}
			if a == "--keep" {
				clear = false
			}
		}
		path := "/v1/inbox"
		if wait != "" {
			path += "?wait=" + wait
		} else if !clear {
			path += "?clear=0"
		}
		printJSON(c.get(path))
	case "chat":
		// interactive one-liner: connect + send + wait inbox
		if len(args) < 4 {
			fatal("usage: coe-cli chat <device_id> <host:port> <text...>")
		}
		printJSON(c.post("/v1/sessions", map[string]string{"device_id": args[1], "addr": args[2]}))
		text := strings.Join(args[3:], " ")
		printJSON(c.post("/v1/send", map[string]string{"to": args[1], "text": text}))
		time.Sleep(300 * time.Millisecond)
		printJSON(c.get("/v1/inbox?wait=3s"))
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `coe-cli — talk to local coe-node app API

  coe-cli [-api URL] [-token TOK] <cmd>

Commands:
  health
  status
  peers
  peers add <device_id> <host:port>
  connect <device_id> [host:port]
  webrtc dial|accept <device_id> <signal_url>
  sessions
  send <device_id> <text...>
  inbox [--wait=5s] [--keep]
  chat <device_id> <host:port> <text...>
`)
}

type client struct {
	base, token string
}

func (c *client) get(path string) any {
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		fatal(err.Error())
	}
	return c.do(req)
}

func (c *client) post(path string, body any) any {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(b))
	if err != nil {
		fatal(err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

func (c *client) do(req *http.Request) any {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fatal(fmt.Sprintf("http %d: %s", resp.StatusCode, string(data)))
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return string(data)
	}
	return v
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

func fatal(s string) {
	fmt.Fprintln(os.Stderr, s)
	os.Exit(1)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
