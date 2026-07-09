#!/usr/bin/env bash
# Server-side update script — run this on the server after uploading new binary
# Usage: bash server-update.sh [provider-name]

set -euo pipefail

PROVIDER="${1:-deepseek_for_server}"
INSTALL_DIR="${INSTALL_DIR:-/opt/cc-switch}"
CCSWITCH="${INSTALL_DIR}/cc-switch"

echo "=== cc-switch-server update ==="

# 1. Kill old proxy daemon
if [ -f ~/.cc-switch-server/proxy.pid ]; then
    PID=$(head -1 ~/.cc-switch-server/proxy.pid 2>/dev/null || true)
    if [ -n "${PID}" ] && kill -0 "${PID}" 2>/dev/null; then
        echo "Killing old proxy daemon (PID ${PID})..."
        kill -9 "${PID}" 2>/dev/null || true
    fi
    rm -f ~/.cc-switch-server/proxy.pid
fi
pkill -9 -f 'cc-switch proxy-daemon' 2>/dev/null || true
sleep 1

# 2. Verify binary exists
if [ ! -f "${CCSWITCH}" ]; then
    echo "ERROR: ${CCSWITCH} not found. Upload it first."
    exit 1
fi
chmod +x "${CCSWITCH}"

# 3. Switch provider (auto-starts new proxy daemon)
echo "Switching to ${PROVIDER}..."
"${CCSWITCH}" set "${PROVIDER}"

# 4. Verify
sleep 1
echo ""
"${CCSWITCH}" proxy-status
echo ""
echo "=== Done. Run 'claude' to start. ==="
