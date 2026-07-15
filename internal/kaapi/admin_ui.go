package kaapi

import (
	"net/http"
)

// Minimal operator HTML UI (Phase 2). Uses Bearer token from localStorage.
func (s *Server) handleAdminUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(adminHTML))
}

// Plain JS only (no template literals) so this stays a Go raw string.
const adminHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>COE Admin</title>
<style>
body{font-family:system-ui,sans-serif;max-width:960px;margin:1.5rem auto;padding:0 1rem;background:#0f1419;color:#e7ecf1}
h1,h2{font-weight:600}
input,button{font:inherit;padding:.4rem .6rem;margin:.2rem 0;border-radius:6px;border:1px solid #334}
input{background:#1a2332;color:#e7ecf1;width:min(100%,28rem)}
button{background:#2b6cb0;color:#fff;border:none;cursor:pointer}
button.secondary{background:#334}
button.danger{background:#9b2c2c}
.card{background:#1a2332;border:1px solid #2a3544;border-radius:10px;padding:1rem;margin:1rem 0}
table{width:100%;border-collapse:collapse;font-size:.9rem}
th,td{text-align:left;padding:.4rem;border-bottom:1px solid #2a3544}
.err{color:#fc8181}.ok{color:#68d391} code{background:#0f1419;padding:.1rem .3rem;border-radius:4px}
.row{display:flex;flex-wrap:wrap;gap:.5rem;align-items:center}
.muted{color:#8b9bb0;font-size:.85rem}
</style>
</head>
<body>
<h1>COE Admin</h1>
<p class="muted">Self-hosted Key Authority. Paste admin Bearer token (browser only). Open over HTTPS to your KA.</p>
<div class="card">
  <label>Admin token</label><br/>
  <input id="tok" type="password" placeholder="Bearer token" autocomplete="off"/>
  <div class="row">
    <button type="button" id="btnSave">Save in browser</button>
    <button type="button" class="secondary" id="btnRefresh">Refresh</button>
  </div>
  <p id="msg" class="muted"></p>
</div>
<div class="card">
  <h2>Create voucher</h2>
  <div class="row">
    <input id="vlabel" placeholder="label (optional)"/>
    <input id="vmax" type="number" value="1" min="1" style="width:5rem" title="max uses"/>
    <input id="vttl" type="number" value="168" min="0" style="width:5rem" title="ttl hours"/>
    <button type="button" id="btnMint">Mint</button>
  </div>
  <pre id="vcode" class="ok"></pre>
</div>
<div class="card">
  <h2>Devices</h2>
  <div id="devices"></div>
</div>
<div class="card">
  <h2>Vouchers</h2>
  <div id="vouchers"></div>
</div>
<script>
(function(){
function el(id){ return document.getElementById(id); }
function tok(){ return localStorage.getItem("coe_admin_tok") || el("tok").value.trim(); }
function msg(t, cls){ var e=el("msg"); e.textContent=t; e.className=cls||"muted"; }
function headers(){
  var t=tok();
  var h={"Content-Type":"application/json"};
  if(t) h["Authorization"]="Bearer "+t;
  return h;
}
function esc(s){
  return String(s).replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;").replace(/"/g,"&quot;");
}
function api(path, opt){
  opt = opt || {};
  opt.headers = headers();
  return fetch(path, opt).then(function(r){
    return r.text().then(function(text){
      var j; try{ j=JSON.parse(text); }catch(e){ j={raw:text}; }
      if(!r.ok) throw new Error(j.error || (r.status+" "+text));
      return j;
    });
  });
}
function renderDevices(list){
  if(!list || !list.length){ el("devices").innerHTML="<p class=\"muted\">No devices</p>"; return; }
  var h="<table><tr><th>ID</th><th>Profile</th><th>Revoked</th><th>Serial</th><th></th></tr>";
  for(var i=0;i<list.length;i++){
    var d=list[i];
    h+="<tr><td><code>"+esc(d.device_id)+"</code></td><td>"+esc(d.profile||"")+"</td><td>"+(d.revoked?"yes":"no")+"</td>";
    h+="<td>"+(d.last_serial||"")+"</td><td>";
    if(!d.revoked) h+="<button type=\"button\" class=\"danger\" data-dev=\""+esc(d.device_id)+"\">Revoke</button>";
    h+="</td></tr>";
  }
  el("devices").innerHTML=h+"</table>";
  el("devices").querySelectorAll("button[data-dev]").forEach(function(b){
    b.onclick=function(){ revokeDev(b.getAttribute("data-dev")); };
  });
}
function renderVouchers(list){
  if(!list || !list.length){ el("vouchers").innerHTML="<p class=\"muted\">No vouchers</p>"; return; }
  var h="<table><tr><th>ID</th><th>Label</th><th>Uses</th><th>Expires</th><th>Revoked</th><th></th></tr>";
  for(var i=0;i<list.length;i++){
    var v=list[i];
    h+="<tr><td><code>"+esc(v.id)+"</code></td><td>"+esc(v.label||"")+"</td><td>"+v.uses+"/"+v.max_uses+"</td>";
    h+="<td>"+(v.expires_at||"—")+"</td><td>"+(v.revoked?"yes":"no")+"</td><td>";
    if(!v.revoked) h+="<button type=\"button\" class=\"danger\" data-vid=\""+esc(v.id)+"\">Revoke</button>";
    h+="</td></tr>";
  }
  el("vouchers").innerHTML=h+"</table>";
  el("vouchers").querySelectorAll("button[data-vid]").forEach(function(b){
    b.onclick=function(){ revokeV(b.getAttribute("data-vid")); };
  });
}
function loadAll(){
  el("tok").value = localStorage.getItem("coe_admin_tok")||"";
  Promise.all([api("/v1/devices"), api("/v1/vouchers")]).then(function(arr){
    renderDevices(arr[0].devices||[]);
    renderVouchers(arr[1].vouchers||[]);
    msg("Loaded.","ok");
  }).catch(function(e){ msg(String(e.message||e),"err"); });
}
function createVoucher(){
  var body={
    label: el("vlabel").value,
    max_uses: parseInt(el("vmax").value,10)||1,
    ttl_hours: parseInt(el("vttl").value,10)||168,
    profile: "strong"
  };
  api("/v1/vouchers",{method:"POST",body:JSON.stringify(body)}).then(function(j){
    el("vcode").textContent="code (copy once): "+j.code+"\nid: "+j.id;
    msg("Voucher created.","ok");
    loadAll();
  }).catch(function(e){ msg(String(e.message||e),"err"); });
}
function revokeDev(id){
  if(!confirm("Revoke "+id+"?")) return;
  api("/v1/revoke",{method:"POST",body:JSON.stringify({device_id:id,reason:"admin-ui"})})
    .then(loadAll).catch(function(e){ msg(String(e.message||e),"err"); });
}
function revokeV(id){
  if(!confirm("Revoke voucher "+id+"?")) return;
  api("/v1/vouchers/revoke",{method:"POST",body:JSON.stringify({id:id})})
    .then(loadAll).catch(function(e){ msg(String(e.message||e),"err"); });
}
el("btnSave").onclick=function(){
  localStorage.setItem("coe_admin_tok", el("tok").value.trim());
  msg("Token saved in this browser only.","ok");
};
el("btnRefresh").onclick=loadAll;
el("btnMint").onclick=createVoucher;
el("tok").value = localStorage.getItem("coe_admin_tok")||"";
})();
</script>
</body>
</html>
`