#!/usr/bin/env bash
# Install coe-node as a per-user launchd agent (macOS).
# Usage:
#   ./scripts/install-agent-macos.sh -d alice -k https://ka.example.com -c /path/ka.crt [-v VOUCHER] [-e]
set -euo pipefail

DEVICE_ID=""
KA_URL="https://127.0.0.1:8443"
KA_CA=""
VOUCHER=""
ENROLL=0
LISTEN="0.0.0.0:9001"
API="127.0.0.1:7701"
API_TOKEN=""
INSTALL_ROOT="${HOME}/Library/Application Support/COE"
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
    h|*) echo "Usage: $0 -d device_id [-k ka_url] [-c ka.crt] [-v voucher] [-e enroll]"; exit 1 ;;
  esac
done

if [[ -z "$DEVICE_ID" ]]; then
  echo "device id required (-d)" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if [[ -z "$SOURCE_BIN" ]]; then
  SOURCE_BIN="$ROOT/bin"
fi

BIN_DIR="$INSTALL_ROOT/bin"
DEV_DIR="$INSTALL_ROOT/devices/$DEVICE_ID"
LOG_DIR="$INSTALL_ROOT/logs"
CFG="$DEV_DIR/node.json"
ID_PATH="$DEV_DIR/identity.json"
STORE="$DEV_DIR/keys"
LAUNCH_DIR="${HOME}/Library/LaunchAgents"
LABEL="com.coe.node.${DEVICE_ID}"
PLIST="$LAUNCH_DIR/${LABEL}.plist"

mkdir -p "$BIN_DIR" "$DEV_DIR" "$STORE" "$LOG_DIR" "$LAUNCH_DIR"

copy_bin() {
  local name="$1"
  if [[ -f "$SOURCE_BIN/$name" ]]; then
    cp -f "$SOURCE_BIN/$name" "$BIN_DIR/$name"
    chmod +x "$BIN_DIR/$name"
  elif [[ -f "$SOURCE_BIN/${name}-darwin-amd64" ]]; then
    cp -f "$SOURCE_BIN/${name}-darwin-amd64" "$BIN_DIR/$name"
    chmod +x "$BIN_DIR/$name"
  elif [[ -f "$SOURCE_BIN/${name}-darwin-arm64" ]]; then
    cp -f "$SOURCE_BIN/${name}-darwin-arm64" "$BIN_DIR/$name"
    chmod +x "$BIN_DIR/$name"
  else
    echo "missing $name in $SOURCE_BIN" >&2
    exit 1
  fi
}

copy_bin coe-node
[[ -f "$SOURCE_BIN/coe-keygen" || -f "$SOURCE_BIN/coe-keygen-darwin-arm64" || -f "$SOURCE_BIN/coe-keygen-darwin-amd64" ]] && copy_bin coe-keygen || true
[[ -f "$SOURCE_BIN/coe-cli" || -f "$SOURCE_BIN/coe-cli-darwin-arm64" || -f "$SOURCE_BIN/coe-cli-darwin-amd64" ]] && copy_bin coe-cli || true

if [[ -z "$API_TOKEN" ]]; then
  API_TOKEN="$(openssl rand -hex 16)"
  echo "Generated API token (save it): $API_TOKEN"
fi

if [[ ! -f "$ID_PATH" ]]; then
  "$BIN_DIR/coe-keygen" -device-id "$DEVICE_ID" -out "$ID_PATH"
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
  if [[ -z "$VOUCHER" ]]; then
    echo "enroll needs -v voucher" >&2
    exit 1
  fi
  "$BIN_DIR/coe-node" -config "$CFG" -enroll -voucher "$VOUCHER"
fi

# unload old
launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true

cat >"$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${BIN_DIR}/coe-node</string>
    <string>-config</string>
    <string>${CFG}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>${LOG_DIR}/${DEVICE_ID}.out.log</string>
  <key>StandardErrorPath</key>
  <string>${LOG_DIR}/${DEVICE_ID}.err.log</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>COE_ICE_POLICY</key>
    <string>all</string>
  </dict>
</dict>
</plist>
EOF

launchctl bootstrap "gui/$(id -u)" "$PLIST"
launchctl enable "gui/$(id -u)/$LABEL"
launchctl kickstart -k "gui/$(id -u)/$LABEL"

echo "Installed LaunchAgent: $LABEL"
echo "Config: $CFG"
echo "API: http://$API  token: $API_TOKEN"
echo "Logs: $LOG_DIR"
echo "UI: coe-tray -api http://$API -token $API_TOKEN"
echo "Uninstall: launchctl bootout gui/\$(id -u)/$LABEL; rm -f $PLIST"
