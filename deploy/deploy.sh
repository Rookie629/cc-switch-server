#!/usr/bin/env bash
# Local build + deploy script — run from project root
# Usage: bash deploy/deploy.sh <server-ip> [provider-name]

set -euo pipefail

SERVER="${1:-}"
PROVIDER="${2:-deepseek_for_server}"
SERVER_DIR="${SERVER_DIR:-/opt/cc-switch}"

if [ -z "${SERVER}" ]; then
    echo "Usage: bash deploy/deploy.sh <server-ip|host> [provider-name]"
    echo "  e.g. bash deploy/deploy.sh 10.0.0.5 deepseek_for_server"
    exit 1
fi

echo "=== Build cc-switch ==="
cd "$(dirname "$0")/.."
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o cc-switch .
echo "  Built: $(du -h cc-switch | cut -f1)"

echo ""
echo "=== Upload to ${SERVER} ==="
scp cc-switch "deploy/server-update.sh" "web/"* "${SERVER}:${SERVER_DIR}/"

echo ""
echo "=== Run server update ==="
ssh "${SERVER}" "cd ${SERVER_DIR} && bash server-update.sh ${PROVIDER}"

echo ""
echo "=== Cleanup ==="
rm -f cc-switch
echo "Done."
