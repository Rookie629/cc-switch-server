#!/usr/bin/env bash
# Local build + deploy — run from project root
# Usage: bash deploy/deploy.sh [provider-name]

set -euo pipefail

# --- Server config ---
SERVER="root@10.15.89.242"
PORT="22793"
SERVER_DIR="/opt/cc-switch"
# ---------------------

PROVIDER="${1:-deepseek_for_server}"

SSH_OPTS="-p ${PORT}"
SCP_OPTS="-P ${PORT}"

echo "=== Build cc-switch ==="
cd "$(dirname "$0")/.."
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o cc-switch .
echo "  Built: $(du -h cc-switch | cut -f1)"

echo ""
echo "=== Upload to ${SERVER}:${PORT} ==="
scp ${SCP_OPTS} cc-switch "deploy/server-update.sh" "web/"* "${SERVER}:${SERVER_DIR}/"

echo ""
echo "=== Run server update ==="
ssh ${SSH_OPTS} "${SERVER}" "cd ${SERVER_DIR} && bash server-update.sh ${PROVIDER}"

echo ""
echo "=== Cleanup ==="
rm -f cc-switch
echo "Done."
