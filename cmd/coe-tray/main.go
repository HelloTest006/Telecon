// coe-tray: simple desktop UI over coe-node local API (opens browser).
// Apache-2.0 client tooling.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func main() {
	api := flag.String("api", envOr("COE_API", "http://127.0.0.1:7701"), "coe-node local API base")
	token := flag.String("token", envOr("COE_API_TOKEN", ""), "Bearer token for local API")
	listen := flag.String("listen", "127.0.0.1:0", "tray UI listen (loopback)")
	noOpen := flag.Bool("no-open", false, "do not open browser")
	flag.Parse()

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	addr := ln.Addr().String()
	uiURL := "http://" + addr + "/"
	log.Printf("COE tray UI %s → agent %s", uiURL, *api)

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/proxy/", func(w http.ResponseWriter, r *http.Request) {
		proxy(w, r, strings.TrimRight(*api, "/"), *token)
	})

	go func() {
		if err := http.Serve(ln, mux); err != nil {
			log.Printf("ui server: %v", err)
		}
	}()

	if !*noOpen {
		time.Sleep(200 * time.Millisecond)
		if err := openBrowser(uiURL); err != nil {
			log.Printf("open browser: %v — open %s manually", err, uiURL)
		}
	}

	select {}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func proxy(w http.ResponseWriter, r *http.Request, apiBase, token string) {
	path := strings.TrimPrefix(r.URL.Path, "/proxy")
	if path == "" {
		path = "/"
	}
	url := apiBase + path
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	var body io.Reader
	if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
		b, _ := io.ReadAll(r.Body)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(r.Method, url, body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	for k, vv := range resp.Header {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, trayHTML)
}

const trayHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>COE Tray</title>
<style>
body{font-family:system-ui,sans-serif;max-width:720px;margin:1rem auto;padding:0 1rem;background:#0f1419;color:#e7ecf1}
h1{font-size:1.25rem} .card{background:#1a2332;border:1px solid #2a3544;border-radius:10px;padding:1rem;margin:1rem 0}
input,button,textarea{font:inherit;padding:.4rem .6rem;border-radius:6px;border:1px solid #334;background:#0f1419;color:#e7ecf1}
button{background:#2b6cb0;border:none;cursor:pointer;color:#fff}
button.secondary{background:#334}
.row{display:flex;flex-wrap:wrap;gap:.5rem;align-items:center;margin:.4rem 0}
.muted{color:#8b9bb0;font-size:.85rem} .err{color:#fc8181} .ok{color:#68d391}
pre{white-space:pre-wrap;word-break:break-word;font-size:.85rem}
#log{max-height:12rem;overflow:auto;background:#0f1419;padding:.5rem;border-radius:6px}
</style>
</head>
<body>
<h1>COE desktop UI</h1>
<p class="muted">Talks to local <code>coe-node</code> only (127.0.0.1). Messages never go through this page to the internet except via your agent.</p>
<div class="card">
  <div class="row"><strong>Status</strong> <button type="button" id="btnRefresh">Refresh</button></div>
  <pre id="status" class="muted">loading…</pre>
</div>
<div class="card">
  <h2>Connect (TCP)</h2>
  <div class="row">
    <input id="peerId" placeholder="peer device_id"/>
    <input id="peerAddr" placeholder="host:port"/>
    <button type="button" id="btnConnect">Connect</button>
  </div>
  <h2>Connect (WebRTC)</h2>
  <div class="row">
    <input id="rtcPeer" placeholder="peer device_id"/>
    <input id="signalUrl" placeholder="http://signal:8450"/>
    <select id="rtcRole"><option value="dial">dial</option><option value="accept">accept</option></select>
    <button type="button" id="btnRTC">WebRTC</button>
  </div>
</div>
<div class="card">
  <h2>Send</h2>
  <div class="row">
    <input id="to" placeholder="to device_id"/>
    <input id="text" placeholder="message" style="flex:1;min-width:12rem"/>
    <button type="button" id="btnSend">Send</button>
  </div>
</div>
<div class="card">
  <h2>Inbox</h2>
  <div class="row">
    <button type="button" id="btnInbox">Fetch</button>
    <button type="button" class="secondary" id="btnPoll">Poll 15s</button>
  </div>
  <pre id="inbox" class="muted"></pre>
</div>
<div class="card">
  <h2>Log</h2>
  <div id="log" class="muted"></div>
</div>
<script>
(function(){
function $(id){ return document.getElementById(id); }
function log(m, cls){
  var d=document.createElement("div");
  d.className=cls||"";
  d.textContent=new Date().toISOString().slice(11,19)+" "+m;
  $("log").prepend(d);
}
function api(method, path, body){
  var opt={method:method, headers:{}};
  if(body!==undefined){ opt.headers["Content-Type"]="application/json"; opt.body=JSON.stringify(body); }
  return fetch("/proxy"+path, opt).then(function(r){
    return r.text().then(function(t){
      var j; try{ j=JSON.parse(t); }catch(e){ j={raw:t}; }
      if(!r.ok) throw new Error(j.error||(r.status+" "+t));
      return j;
    });
  });
}
function refresh(){
  api("GET","/v1/status").then(function(j){
    $("status").textContent=JSON.stringify(j,null,2);
    $("status").className="ok";
    log("status ok","ok");
  }).catch(function(e){
    $("status").textContent=String(e.message||e);
    $("status").className="err";
    log("status fail: "+e.message,"err");
  });
}
$("btnRefresh").onclick=refresh;
$("btnConnect").onclick=function(){
  api("POST","/v1/sessions",{device_id:$("peerId").value, addr:$("peerAddr").value})
    .then(function(){ log("connected TCP","ok"); refresh(); })
    .catch(function(e){ log(e.message,"err"); });
};
$("btnRTC").onclick=function(){
  api("POST","/v1/sessions/webrtc",{
    device_id:$("rtcPeer").value,
    signal_url:$("signalUrl").value,
    role:$("rtcRole").value,
    wait_sec:60
  }).then(function(){ log("webrtc session ok","ok"); refresh(); })
    .catch(function(e){ log(e.message,"err"); });
};
$("btnSend").onclick=function(){
  api("POST","/v1/send",{to:$("to").value, text:$("text").value})
    .then(function(){ log("sent","ok"); $("text").value=""; })
    .catch(function(e){ log(e.message,"err"); });
};
function showInbox(j){
  var msgs=j.messages||[];
  $("inbox").textContent=msgs.length?JSON.stringify(msgs,null,2):"(empty)";
}
$("btnInbox").onclick=function(){
  api("GET","/v1/inbox").then(showInbox).catch(function(e){ log(e.message,"err"); });
};
$("btnPoll").onclick=function(){
  log("polling inbox 15s…");
  api("GET","/v1/inbox?wait=15s").then(showInbox).catch(function(e){ log(e.message,"err"); });
};
refresh();
setInterval(refresh, 10000);
})();
</script>
</body>
</html>
`