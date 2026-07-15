#!/usr/bin/env bash
# Remove macOS LaunchAgent.
# Usage: ./scripts/uninstall-agent-macos.sh -d alice [-r]
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
LABEL="com.coe.node.${DEVICE_ID}"
PLIST="${HOME}/Library/LaunchAgents/${LABEL}.plist"
launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
rm -f "$PLIST"
if [[ "$REMOVE_DATA" -eq 1 ]]; then
  rm -rf "${HOME}/Library/Application Support/COE/devices/$DEVICE_ID"
  echo "removed device data"
fi
echo "uninstalled $LABEL"
