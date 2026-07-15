#!/usr/bin/env bash
# Remove Linux user systemd agent.
# Usage: ./scripts/uninstall-agent-linux.sh -d alice [-r remove data]
set -euo pipefail
DEVICE_ID=""
REMOVE_DATA=0
while getopts "d:rh" opt; do
  case "$opt" in
    d) DEVICE_ID="$OPTARG" ;;
    r) REMOVE_DATA=1 ;;
    *) echo "Usage: $0 -d device_id [-r]"; exit 1 ;;
  esac
done
[[ -n "$DEVICE_ID" ]] || { echo "need -d"; exit 1; }
UNIT="coe-node-${DEVICE_ID}.service"
UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
systemctl --user disable --now "$UNIT" 2>/dev/null || true
rm -f "$UNIT_DIR/$UNIT"
systemctl --user daemon-reload 2>/dev/null || true
if [[ "$REMOVE_DATA" -eq 1 ]]; then
  rm -rf "${XDG_DATA_HOME:-$HOME/.local/share}/coe/devices/$DEVICE_ID"
  echo "removed device data"
fi
echo "uninstalled $UNIT"
