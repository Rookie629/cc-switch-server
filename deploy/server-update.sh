#!/usr/bin/env bash
# Server-side update script — run on server after uploading new binary
# Usage: bash server-update.sh [provider-name]

set -euo pipefail

PROVIDER="${1:-deepseek_for_server}"
BIN_DIR="/opt/cc-switch"
CCSWITCH="${BIN_DIR}/cc-switch"

echo "=== cc-switch-server update ==="

# 0. Copy web files if they were uploaded alongside the binary
if [ -f "${BIN_DIR}/index.html" ]; then
    mkdir -p "${BIN_DIR}/web"
    cp "${BIN_DIR}"/index.html "${BIN_DIR}"/app.js "${BIN_DIR}"/style.css "${BIN_DIR}/web/" 2>/dev/null || true
    echo "  Web files synced."
fi

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
    echo "ERROR: ${CCSWITCH} not found. Upload it first:"
    echo "  scp -P 22793 ~/cc-switch-server/cc-switch root@10.15.89.242:${BIN_DIR}/"
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
