#!/usr/bin/env bash
# Install coe-node as a per-user systemd user service (Linux).
# Usage:
#   ./scripts/install-agent-linux.sh -d alice -k https://ka.example.com -c /path/ka.crt [-v VOUCHER] [-e]
set -euo pipefail

DEVICE_ID=""
KA_URL="https://127.0.0.1:8443"
KA_CA=""
VOUCHER=""
ENROLL=0
LISTEN="0.0.0.0:9001"
API="127.0.0.1:7701"
API_TOKEN=""
INSTALL_ROOT="${XDG_DATA_HOME:-$HOME/.local/share}/coe"
SOURCE_BIN=""

while getopts "d:k:c:v:el:a:t:s:h" opt; do
  case "$opt" in
    d) DEVICE_ID="$OPTARG" ;;
    k) KA_URL="$OPTARG" ;;
    c) KA_CA="$OPTARG" ;;
    v) VOUCHER="$OPTARG" ;;
    e) ENROLL=1 ;;
    l) LISTEN="$OPTARG" ;;
    a) API="$OPTARG" ;;
    t) API_TOKEN="$OPTARG" ;;
    s) SOURCE_BIN="$OPTARG" ;;
    h|*) echo "Usage: $0 -d device_id [-k ka_url] [-c ka.crt] [-v voucher] [-e enroll] [-l listen] [-a api] [-t api_token]"; exit 1 ;;
  esac
done

if [[ -z "$DEVICE_ID" ]]; then
  echo "device id required (-d)" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if [[ -z "$SOURCE_BIN" ]]; then
  if [[ -x "$ROOT/bin/coe-node" ]]; then
    SOURCE_BIN="$ROOT/bin"
  else
    SOURCE_BIN="$ROOT/bin"
  fi
fi

BIN_DIR="$INSTALL_ROOT/bin"
DEV_DIR="$INSTALL_ROOT/devices/$DEVICE_ID"
LOG_DIR="$INSTALL_ROOT/logs"
CFG="$DEV_DIR/node.json"
ID_PATH="$DEV_DIR/identity.json"
STORE="$DEV_DIR/keys"
UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
UNIT_NAME="coe-node-${DEVICE_ID}.service"

mkdir -p "$BIN_DIR" "$DEV_DIR" "$STORE" "$LOG_DIR" "$UNIT_DIR"

copy_bin() {
  local name="$1"
  if [[ -f "$SOURCE_BIN/$name" ]]; then
    cp -f "$SOURCE_BIN/$name" "$BIN_DIR/$name"
    chmod +x "$BIN_DIR/$name"
  elif [[ -f "$SOURCE_BIN/${name}-linux-amd64" ]]; then
    cp -f "$SOURCE_BIN/${name}-linux-amd64" "$BIN_DIR/$name"
    chmod +x "$BIN_DIR/$name"
  else
    echo "missing $name in $SOURCE_BIN — build: go build -o bin/$name ./cmd/${name#coe-}" >&2
    exit 1
  fi
}

copy_bin coe-node
copy_bin coe-keygen || true
copy_bin coe-cli || true

if [[ -z "$API_TOKEN" ]]; then
  API_TOKEN="$(openssl rand -hex 16 2>/dev/null || head -c 16 /dev/urandom | xxd -p)"
  echo "Generated API token (save it): $API_TOKEN"
fi

if [[ ! -f "$ID_PATH" ]]; then
  if [[ -x "$BIN_DIR/coe-keygen" ]]; then
    "$BIN_DIR/coe-keygen" -device-id "$DEVICE_ID" -out "$ID_PATH"
  else
    echo "identity missing and coe-keygen not found" >&2
    exit 1
  fi
fi

cat >"$CFG" <<EOF
{
  "device_id": "$DEVICE_ID",
  "identity_path": "$ID_PATH",
  "store_dir": "$STORE",
  "ka_url": "$KA_URL",
  "ka_ca_file": "$KA_CA",
  "ka_insecure": false,
  "listen_addr": "$LISTEN",
  "api_addr": "$API",
  "api_token": "$API_TOKEN",
  "profile": "strong",
  "peers": []
}
EOF

if [[ "$ENROLL" -eq 1 ]]; then
  if [[ -n "$VOUCHER" ]]; then
    "$BIN_DIR/coe-node" -config "$CFG" -enroll -voucher "$VOUCHER"
  else
    echo "enroll needs -v voucher" >&2
    exit 1
  fi
fi

cat >"$UNIT_DIR/$UNIT_NAME" <<EOF
[Unit]
Description=COE agent ($DEVICE_ID)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$BIN_DIR/coe-node -config $CFG
Restart=on-failure
RestartSec=3
Environment=COE_STUN=
Environment=COE_TURN_URLS=
Environment=COE_ICE_POLICY=all

[Install]
WantedBy=default.target
EOF

systemctl --user daemon-reload
systemctl --user enable --now "$UNIT_NAME"
# linger so agent can run after logout (optional; needs root once)
if command -v loginctl >/dev/null 2>&1; then
  loginctl enable-linger "$USER" 2>/dev/null || true
fi

echo "Installed user service: $UNIT_NAME"
echo "Config: $CFG"
echo "API: http://$API  token: $API_TOKEN"
echo "Status: systemctl --user status $UNIT_NAME"
echo "UI: coe-tray -api http://$API -token $API_TOKEN"
echo "Uninstall: systemctl --user disable --now $UNIT_NAME; rm -f $UNIT_DIR/$UNIT_NAME"
