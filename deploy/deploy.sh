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

SSH_OPTS="-p ${PORT} -o StrictHostKeyChecking=no"
SCP_OPTS="-P ${PORT} -o StrictHostKeyChecking=no"

# Auto-detect sshpass if installed, fall back to interactive
if command -v sshpass &>/dev/null && [ -n "${SSHPASS:-}" ]; then
    SCP_CMD="sshpass -e scp"
    SSH_CMD="sshpass -e ssh"
else
    SCP_CMD="scp"
    SSH_CMD="ssh"
fi

echo "=== Build cc-switch ==="
cd "$(dirname "$0")/.."
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o cc-switch .
echo "  Built: $(du -h cc-switch | cut -f1)"

echo ""
echo "=== Stop remote proxy ==="
${SSH_CMD} ${SSH_OPTS} "${SERVER}" "pkill -9 -f 'cc-switch proxy-daemon' 2>/dev/null; rm -f ~/.cc-switch-server/proxy.pid; echo '  proxy stopped.'" || true
sleep 1

echo ""
echo "=== Upload to ${SERVER}:${PORT} ==="
${SCP_CMD} ${SCP_OPTS} cc-switch "deploy/server-update.sh" "web/"* "${SERVER}:${SERVER_DIR}/"

echo ""
echo "=== Run server update ==="
${SSH_CMD} ${SSH_OPTS} "${SERVER}" "cd ${SERVER_DIR} && bash server-update.sh ${PROVIDER}"

echo ""
echo "=== Cleanup ==="
rm -f cc-switch
echo "Done."
