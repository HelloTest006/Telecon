#!/bin/bash
# Simple KA backup script. Run daily via cron.
# Usage: ./backup-ka.sh /path/to/data /path/to/backup/dir

set -euo pipefail
DATA_DIR=${1:-/data}
BACKUP_DIR=${2:-/backups}
DATE=$(date +%Y%m%d-%H%M%S)
mkdir -p "$BACKUP_DIR"

if [ ! -d "$DATA_DIR" ]; then
  echo "DATA_DIR not found: $DATA_DIR" >&2
  exit 1
fi

ARCHIVE="$BACKUP_DIR/ka-$DATE.tar.gz"
tar -czf "$ARCHIVE" -C "$(dirname "$DATA_DIR")" "$(basename "$DATA_DIR")"

echo "Backup created: $ARCHIVE"

# Optional: keep last 30
find "$BACKUP_DIR" -name 'ka-*.tar.gz' -mtime +30 -delete
