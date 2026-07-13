# Self-host COE without Docker

This guide shows how to run **your own** COE servers on a normal computer or VPS.

**No Docker.**  
**You host the servers** — this project does not run a public auth/key service for you.

---

## What you are setting up (simple picture)

```text
  [ Your server ]                    [ Each person's PC ]
  ┌─────────────────┐                ┌──────────────────┐
  │ Key Authority   │  daily keys    │  COE agent       │
  │ (ka)            │◄───────────────│  (coe-node)      │
  │                 │                │                  │
  │ Signaling       │  call setup    │  talks to peers  │
  │ (coe-signal)    │  only          │  directly        │
  └─────────────────┘                └──────────────────┘
```

| Piece | What it does | Who runs it |
|-------|----------------|-------------|
| **Key Authority (`ka`)** | Gives each device a new key every day; enrolls/revokes devices | **You** (server) |
| **Signaling (`coe-signal`)** | Helps two devices find each other for WebRTC (does **not** read chat) | **You** (server) |
| **Agent (`coe-node`)** | App on each PC that encrypts and talks peer-to-peer | Each device owner |
| **TURN (optional)** | Helps when both sides are behind hard firewalls | **You** if needed |

**Messages do not go through your key server.** The key server only hands out daily keys.

---

## Before you start

### Skills you need
- Open a terminal (Command Prompt / PowerShell on Windows, Terminal on Linux)
- Copy and paste commands
- Know your computer’s admin password

### What you need
1. **A computer that stays on** (home PC, office server, or cheap VPS)
2. **This software** — either:
   - download from [GitHub Releases](https://github.com/HelloTest006/Telecon/releases), or  
   - build from source if you have Go installed
3. A **long secret password** for the Key Authority (called the admin token)  
   - Example idea: 48 random characters — **not** `password` or `dev-admin-token`
4. For real internet use later: a **domain name** and proper HTTPS certificates  
   - For learning at home, temporary self-signed certificates are OK

### Words used in this guide

| Word | Meaning |
|------|---------|
| **Admin token** | Master password for your Key Authority. Keep it private. |
| **Voucher** | One-time code you give a user so their PC can join **without** the admin token |
| **TLS / HTTPS** | Encrypted connection to your server (padlock in browser terms) |
| **Port** | Number like `8443` or `8450` that a program listens on |

---

## Choose your path

| Your server OS | Go to |
|----------------|--------|
| **Linux** (Ubuntu, Debian, etc.) | [Part A — Linux](#part-a--linux) |
| **Windows** | [Part B — Windows](#part-b--windows) |

After the servers run, both paths use the same steps for **vouchers** and **device install**:  
[Part C — Finish setup (both OS)](#part-c--finish-setup-both-os)

---

# Part A — Linux

## A1. Create folders

```bash
sudo mkdir -p /opt/coe/bin /opt/coe/data/tls /opt/coe/logs
sudo chown -R "$USER":"$USER" /opt/coe
```

## A2. Get the programs

### Option 1 — Download release (easiest)

1. Open: https://github.com/HelloTest006/Telecon/releases  
2. Download (linux amd64), for example:
   - `ka-linux-amd64`
   - `coe-signal-linux-amd64`
   - `coe-admin-linux-amd64`
   - `ka-check-linux-amd64`
3. Install them:

```bash
cd ~/Downloads   # or wherever you saved the files
chmod +x ka-linux-amd64 coe-signal-linux-amd64 coe-admin-linux-amd64 ka-check-linux-amd64
mv ka-linux-amd64 /opt/coe/bin/ka
mv coe-signal-linux-amd64 /opt/coe/bin/coe-signal
mv coe-admin-linux-amd64 /opt/coe/bin/coe-admin
mv ka-check-linux-amd64 /opt/coe/bin/ka-check
```

### Option 2 — Build from source

```bash
git clone https://github.com/HelloTest006/Telecon.git
cd Telecon
go build -o /opt/coe/bin/ka ./cmd/ka
go build -o /opt/coe/bin/coe-signal ./cmd/coe-signal
go build -o /opt/coe/bin/coe-admin ./cmd/coe-admin
go build -o /opt/coe/bin/ka-check ./cmd/ka-check
```

## A3. Create your admin token

```bash
export COE_KA_ADMIN_TOKEN=$(openssl rand -hex 24)
echo "SAVE THIS TOKEN SOMEWHERE SAFE:"
echo "$COE_KA_ADMIN_TOKEN"
```

Write it down offline. If you lose it, you must set a new one and update your server.

Also set data paths (copy-paste as a block):

```bash
export COE_KA_MASTER_FILE=/opt/coe/data/master.key
export COE_KA_REGISTRY=/opt/coe/data/registry.json
export COE_KA_TLS_CERT=/opt/coe/data/tls/server.crt
export COE_KA_TLS_KEY=/opt/coe/data/tls/server.key
export COE_KA_TLS_HOSTS=localhost,127.0.0.1
```

For a real domain later, add it to `COE_KA_TLS_HOSTS`, e.g. `ka.example.com,localhost,127.0.0.1`.

## A4. Start the Key Authority

Open a terminal and leave it running:

```bash
/opt/coe/bin/ka \
  -listen 0.0.0.0:8443 \
  -master "$COE_KA_MASTER_FILE" \
  -registry "$COE_KA_REGISTRY" \
  -tls-cert "$COE_KA_TLS_CERT" \
  -tls-key "$COE_KA_TLS_KEY" \
  -admin-token "$COE_KA_ADMIN_TOKEN" \
  -rate-limit 60
```

**First run** creates `master.key` automatically.  
**Back that file up** (USB, encrypted drive, etc.). If you lose it, devices cannot use old daily-key history correctly.

You should see a log line that the Key Authority is listening with TLS.

### Optional: run as a service (always on)

Create `/etc/coe/ka.env` (root only):

```bash
sudo mkdir -p /etc/coe
sudo nano /etc/coe/ka.env
```

Contents (use **your** token):

```text
COE_KA_ADMIN_TOKEN=paste-your-long-token-here
```

```bash
sudo chmod 600 /etc/coe/ka.env
```

Create `/etc/systemd/system/coe-ka.service`:

```ini
[Unit]
Description=COE Key Authority
After=network.target

[Service]
Type=simple
User=YOUR_LINUX_USER
EnvironmentFile=/etc/coe/ka.env
WorkingDirectory=/opt/coe
ExecStart=/opt/coe/bin/ka -listen 0.0.0.0:8443 -master /opt/coe/data/master.key -registry /opt/coe/data/registry.json -tls-cert /opt/coe/data/tls/server.crt -tls-key /opt/coe/data/tls/server.key -admin-token ${COE_KA_ADMIN_TOKEN} -rate-limit 60
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Replace `YOUR_LINUX_USER` with your username. Then:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now coe-ka
sudo systemctl status coe-ka
```

## A5. Start signaling

In a **second** terminal (or second service):

```bash
/opt/coe/bin/coe-signal -listen 0.0.0.0:8450
```

This only helps devices set up WebRTC. It does **not** see message content.

Optional service file `/etc/systemd/system/coe-signal.service`:

```ini
[Unit]
Description=COE signaling (setup only, not chat)
After=network.target

[Service]
Type=simple
User=YOUR_LINUX_USER
ExecStart=/opt/coe/bin/coe-signal -listen 0.0.0.0:8450
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now coe-signal
```

## A6. Quick health check

```bash
curl -sk https://127.0.0.1:8443/v1/health
```

You want JSON that looks roughly like `"status":"ok"`.

Full check (use the same admin token):

```bash
/opt/coe/bin/ka-check \
  -url https://127.0.0.1:8443 \
  -ca /opt/coe/data/tls/server.crt \
  -admin-token "$COE_KA_ADMIN_TOKEN"
```

Exit code **0** means good enough to continue.  
If it fails on admin token, you still used a default/weak token — fix that first.

**Continue at [Part C](#part-c--finish-setup-both-os).**

---

# Part B — Windows

Use **PowerShell** (right‑click Start → Windows PowerShell or Terminal).

## B1. Create folders

```powershell
New-Item -ItemType Directory -Force -Path C:\coe\bin, C:\coe\data\tls, C:\coe\logs | Out-Null
```

## B2. Get the programs

### Option 1 — Download release

1. Open https://github.com/HelloTest006/Telecon/releases  
2. Download Windows files, for example:
   - `ka` is often built as Linux on the release page; if you only see Linux KA, use **Option 2 (build)** below for `ka` / `coe-signal` / `coe-admin` on Windows, **or** run servers on a Linux VPS and only use Windows for the agent.
3. If you built or obtained `.exe` files, put them here:
   - `C:\coe\bin\ka.exe`
   - `C:\coe\bin\coe-signal.exe`
   - `C:\coe\bin\coe-admin.exe`
   - `C:\coe\bin\ka-check.exe`

### Option 2 — Build on Windows (needs [Go](https://go.dev/dl/))

```powershell
cd $env:USERPROFILE
git clone https://github.com/HelloTest006/Telecon.git
cd Telecon
go build -o C:\coe\bin\ka.exe ./cmd/ka
go build -o C:\coe\bin\coe-signal.exe ./cmd/coe-signal
go build -o C:\coe\bin\coe-admin.exe ./cmd/coe-admin
go build -o C:\coe\bin\ka-check.exe ./cmd/ka-check
```

## B3. Create your admin token

```powershell
$bytes = New-Object byte[] 24
[System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
$env:COE_KA_ADMIN_TOKEN = ($bytes | ForEach-Object { $_.ToString('x2') }) -join ''
Write-Host "SAVE THIS TOKEN SOMEWHERE SAFE:"
Write-Host $env:COE_KA_ADMIN_TOKEN
```

Copy the token to a password manager or paper. Do not put it in a public chat.

```powershell
$env:COE_KA_MASTER_FILE = 'C:\coe\data\master.key'
$env:COE_KA_REGISTRY    = 'C:\coe\data\registry.json'
$env:COE_KA_TLS_CERT    = 'C:\coe\data\tls\server.crt'
$env:COE_KA_TLS_KEY     = 'C:\coe\data\tls\server.key'
$env:COE_KA_TLS_HOSTS   = 'localhost,127.0.0.1'
```

**Note:** Environment variables in PowerShell last for **that window only**. If you open a new window, set them again (or use a small `.ps1` script you keep private).

## B4. Start the Key Authority

Leave this window open:

```powershell
& C:\coe\bin\ka.exe `
  -listen 0.0.0.0:8443 `
  -master $env:COE_KA_MASTER_FILE `
  -registry $env:COE_KA_REGISTRY `
  -tls-cert $env:COE_KA_TLS_CERT `
  -tls-key $env:COE_KA_TLS_KEY `
  -admin-token $env:COE_KA_ADMIN_TOKEN `
  -rate-limit 60
```

First run creates `C:\coe\data\master.key`. **Back it up.**

Allow access if Windows Firewall asks about `ka.exe`.

### Optional: keep it running after logoff

For a home lab, Task Scheduler or [NSSM](https://nssm.cc/) can run `ka.exe` at startup.  
For a real public server, many people use a **Linux VPS** for `ka` and only run the **agent** on Windows PCs.

Example Task Scheduler idea (manual UI):

1. Task Scheduler → Create Task  
2. Run whether user is logged on or not (if appropriate for your security model)  
3. Action: start `C:\coe\bin\ka.exe` with the same arguments as above  
4. Store the admin token in a protected way (not a world-readable script)

## B5. Start signaling

Open a **second** PowerShell window, set the same paths if needed, then:

```powershell
& C:\coe\bin\coe-signal.exe -listen 0.0.0.0:8450
```

Allow firewall for `coe-signal.exe` if prompted.

## B6. Quick health check

New PowerShell window:

```powershell
# Ignore cert warning for local self-signed lab cert
try {
  Invoke-RestMethod -Uri https://127.0.0.1:8443/v1/health -SkipCertificateCheck
} catch {
  # Older PowerShell: use curl.exe
  curl.exe -sk https://127.0.0.1:8443/v1/health
}
```

Full check (paste your token again if this is a new window):

```powershell
& C:\coe\bin\ka-check.exe `
  -url https://127.0.0.1:8443 `
  -ca C:\coe\data\tls\server.crt `
  -admin-token $env:COE_KA_ADMIN_TOKEN
```

Want exit code **0**.

**Continue at [Part C](#part-c--finish-setup-both-os).**

---

# Part C — Finish setup (both OS)

Do this after `ka` (and ideally `coe-signal`) are running.

## C1. Create a voucher (invite code)

A voucher lets a person’s PC join **without** giving them your admin token.

**Linux:**

```bash
/opt/coe/bin/coe-admin \
  -ka https://127.0.0.1:8443 \
  -ka-ca /opt/coe/data/tls/server.crt \
  -admin-token "$COE_KA_ADMIN_TOKEN" \
  voucher -max-uses 1 -ttl-hours 72 -label first-pc
```

**Windows:**

```powershell
& C:\coe\bin\coe-admin.exe `
  -ka https://127.0.0.1:8443 `
  -ka-ca C:\coe\data\tls\server.crt `
  -admin-token $env:COE_KA_ADMIN_TOKEN `
  voucher -max-uses 1 -ttl-hours 72 -label first-pc
```

You will see a line like `code=...`.  
**Copy that code once.** Do not post it publicly. Each code can be limited to one use (`-max-uses 1`).

## C2. Install the agent on a Windows PC (the usual end-user machine)

On the **user PC** (can be the same machine for testing):

1. Download from the release page:
   - `coe-node-windows-amd64.exe`
   - `coe-keygen-windows-amd64.exe`
   - `coe-cli-windows-amd64.exe` (optional, for testing)
2. Copy your server’s certificate file to the PC, e.g. `C:\coe\ka.crt`  
   (from the server: `server.crt` under the tls folder)
3. Open PowerShell in the folder with the `.exe` files.

Generate identity and enroll:

```powershell
.\coe-keygen-windows-amd64.exe -device-id alice
# If the release name differs, use: .\coe-keygen.exe -device-id alice

.\coe-node-windows-amd64.exe -device-id alice -enroll -voucher PASTE_CODE_HERE `
  -ka https://127.0.0.1:8443 `
  -ka-ca C:\coe\ka.crt
```

If the server is on another computer, replace `127.0.0.1` with that computer’s IP or domain, e.g. `https://192.168.1.10:8443`.

### Easier install (logon agent)

From a full git checkout of the repo on the user PC:

```powershell
.\scripts\install-agent.ps1 -DeviceId alice `
  -KaUrl https://YOUR_SERVER:8443 `
  -KaCa C:\path\to\ka.crt `
  -Voucher PASTE_CODE_HERE `
  -StartNow
```

This creates a **user logon** task so keys stay protected with Windows DPAPI.  
Do **not** run the agent as the old “LocalSystem” Windows service for normal users.

## C3. Test that the agent is alive

```powershell
.\coe-cli-windows-amd64.exe -api http://127.0.0.1:7701 -token YOUR_API_TOKEN status
```

(If you used `install-agent.ps1`, it prints the API token.)

## C4. Two devices talking (simple LAN test)

1. Enroll a second device (`bob`) with a **new** voucher.  
2. On bob’s machine, start the agent listening (or use install script).  
3. On alice:

```powershell
.\coe-cli.exe -api http://127.0.0.1:7701 -token TOKEN `
  connect bob 192.168.x.x:9001
.\coe-cli.exe -api http://127.0.0.1:7701 -token TOKEN `
  send bob hello
```

Use bob’s real LAN IP. Both PCs must allow the agent ports in the firewall.

For internet / hard Wi‑Fi, you need **TURN** (next section) and often WebRTC instead of raw TCP.

## C5. Optional: TURN (when direct connection fails)

If devices are on different networks and cannot connect:

1. Install [coturn](https://github.com/coturn/coturn) on a server with a public IP, **or** buy a TURN service.  
2. On **each** agent machine, set:

```text
COE_STUN=stun:YOUR_TURN_HOST:3478
COE_TURN_URLS=turn:YOUR_TURN_HOST:3478
COE_TURN_USER=coe
COE_TURN_PASS=your-turn-password
```

Windows PowerShell (current window):

```powershell
$env:COE_STUN = 'stun:YOUR_TURN_HOST:3478'
$env:COE_TURN_URLS = 'turn:YOUR_TURN_HOST:3478'
$env:COE_TURN_USER = 'coe'
$env:COE_TURN_PASS = 'your-turn-password'
```

Then restart the agent.

---

## Safety rules (please read)

| Do | Don’t |
|----|--------|
| Use a long random admin token | Use `dev-admin-token` in real use |
| Back up `master.key` and `registry.json` | Leave the only copy on one disk |
| Give users **vouchers**, not the admin token | Share admin token with every laptop |
| Use HTTPS/TLS for real internet exposure | Run `-http` on the public internet |
| Keep agent API on `127.0.0.1` only | Open port 7701 to the world |

Lab-only insecure KA (never for real users):

```bash
# Linux example — local testing only
./ka -http -listen 127.0.0.1:8443 -admin-token dev-admin-token
```

---

## Firewall cheat sheet

| Port | Program | Notes |
|------|---------|--------|
| **8443** | Key Authority | HTTPS daily keys |
| **8450** | Signaling | WebRTC setup |
| **9001** (example) | Agent peer | TCP chat path; pick what you configure |
| **3478** + UDP range | TURN | Only if you run TURN |

---

## If something fails

| Problem | What to try |
|---------|-------------|
| `ka-check` fails TLS | Use `-ca` path to your `server.crt`; for lab self-signed that is normal |
| `unauthorized` on enroll | Wrong voucher, expired voucher, or already used |
| Agent cannot reach KA | Firewall, wrong URL, or need correct `-ka-ca` file |
| Two agents cannot connect | Same LAN IP/port? Firewall? Need TURN for internet? |
| Lost admin token | Set a new token when starting `ka` (update service/env); old token stops working |

---

## Related docs

| Topic | Link |
|-------|------|
| Short operator path (Docker + bare metal) | [self-host-quickstart.md](self-host-quickstart.md) |
| Docker-only notes | [coe/10-selfhost.md](coe/10-selfhost.md) |
| Desktop agent | [coe/07-desktop-agent.md](coe/07-desktop-agent.md) |
| Vouchers | [coe/11-vouchers.md](coe/11-vouchers.md) |
| Security for operators | [../SECURITY_SELFHOST.md](../SECURITY_SELFHOST.md) |
| Production checks | [coe/08-prod-ka-checklist.md](coe/08-prod-ka-checklist.md) |

---

## Reminder

COE is **self-hosted open source**.  
**You** run Key Authority and signaling for **your** devices.  
The project authors do not host a shared auth or key server for the public.
