#!/usr/bin/env bash
# cc-switch-server Installation Script
# Usage: curl -fsSL <url>/install.sh | bash
#    or: bash install.sh [--prefix /opt/cc-switch-server] [--data-dir /var/lib/cc-switch-server]

set -euo pipefail

# ---- Configuration ----
PREFIX="${PREFIX:-/opt/cc-switch-server}"
DATA_DIR="${DATA_DIR:-/var/lib/cc-switch-server}"
BINARY_NAME="cc-switch"
RELEASE_URL="${RELEASE_URL:-https://github.com/yourusername/cc-switch/releases}"
VERSION="${VERSION:-latest}"
SYSTEMD_DIR="/etc/systemd/system"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# ---- Parse arguments ----
while [[ $# -gt 0 ]]; do
    case "$1" in
        --prefix) PREFIX="$2"; shift 2 ;;
        --data-dir) DATA_DIR="$2"; shift 2 ;;
        --version) VERSION="$2"; shift 2 ;;
        --no-systemd) NO_SYSTEMD=1; shift ;;
        -h|--help)
            echo "Usage: bash install.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --prefix DIR      Installation directory (default: /opt/cc-switch-server)"
            echo "  --data-dir DIR    Data directory (default: /var/lib/cc-switch-server)"
            echo "  --version VER     Version to install (default: latest)"
            echo "  --no-systemd      Skip systemd service registration"
            echo "  -h, --help        Show this help"
            exit 0
            ;;
        *) echo -e "${RED}Unknown option: $1${NC}"; exit 1 ;;
    esac
done

# ---- Helpers ----
log()  { echo -e "${BLUE}[*]${NC} $*"; }
ok()   { echo -e "${GREEN}[✓]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err()  { echo -e "${RED}[✗]${NC} $*"; exit 1; }

require_root() {
    if [[ "$(id -u)" != "0" ]]; then
        err "This step requires root. Run with: sudo bash install.sh"
    fi
}

# ---- Detect platform ----
detect_platform() {
    local os arch
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) err "Unsupported architecture: $arch" ;;
    esac
    echo "${os}-${arch}"
}

# ---- Install from binary release ----
install_from_release() {
    local platform asset_url
    platform=$(detect_platform)
    asset_url="${RELEASE_URL}/download/${VERSION}/${BINARY_NAME}-${platform}"

    log "Downloading ${BINARY_NAME} ${VERSION} for ${platform}..."
    log "URL: ${asset_url}"

    require_root

    # Download binary
    local tmp_bin
    tmp_bin=$(mktemp)
    if command -v curl &>/dev/null; then
        curl -fsSL "${asset_url}" -o "${tmp_bin}" || err "Download failed. Is the version correct?"
    elif command -v wget &>/dev/null; then
        wget -q "${asset_url}" -O "${tmp_bin}" || err "Download failed. Is the version correct?"
    else
        err "Neither curl nor wget found. Install one of them first."
    fi

    chmod +x "${tmp_bin}"

    # Install
    install_dirs
    cp "${tmp_bin}" "${PREFIX}/${BINARY_NAME}"
    rm -f "${tmp_bin}"

    ok "Binary installed to ${PREFIX}/${BINARY_NAME}"
}

# ---- Install from local build ----
install_local() {
    log "Building from source..."

    require_root

    if ! command -v go &>/dev/null; then
        err "Go is not installed. Install Go 1.21+ or use a pre-built binary."
    fi

    # Find the project root (where go.mod lives)
    local src_dir
    src_dir="$(cd "$(dirname "$0")" && pwd)"
    if [[ ! -f "${src_dir}/go.mod" ]]; then
        err "go.mod not found. Run this script from the cc-switch-server source directory."
    fi

    log "Source: ${src_dir}"

    # Build static binary
    cd "${src_dir}"
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${BINARY_NAME}" .
    [[ -f "${BINARY_NAME}" ]] || err "Build failed."

    # Install
    install_dirs
    cp "${BINARY_NAME}" "${PREFIX}/${BINARY_NAME}"

    # Copy web files
    if [[ -d "${src_dir}/web" ]]; then
        cp -r "${src_dir}/web"/* "${PREFIX}/web/"
    fi
    rm -f "${BINARY_NAME}"

    ok "Binary installed to ${PREFIX}/${BINARY_NAME}"
}

# ---- Create directories ----
install_dirs() {
    mkdir -p "${PREFIX}/web"
    mkdir -p "${DATA_DIR}/backups"
    chmod 755 "${PREFIX}" "${PREFIX}/web"
    chmod 700 "${DATA_DIR}" "${DATA_DIR}/backups"
}

# ---- Install systemd service ----
install_systemd() {
    [[ "${NO_SYSTEMD:-0}" == "1" ]] && return

    if ! command -v systemctl &>/dev/null; then
        warn "systemctl not found — skipping systemd service setup."
        return
    fi

    log "Registering systemd service..."

    require_root

    # Generate service file with correct paths
    cat > "${SYSTEMD_DIR}/cc-switch-server.service" <<SERVICEEOF
[Unit]
Description=cc-switch-server - Claude Code AI Provider Manager
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SUDO_USER:-root}
WorkingDirectory=${PREFIX}
ExecStart=${PREFIX}/cc-switch serve --host 0.0.0.0 --port 9876 --data-dir ${DATA_DIR}
ExecStop=/bin/kill -TERM \$MAIN_PID
KillSignal=SIGTERM
KillMode=mixed
TimeoutStopSec=10
Restart=on-failure
RestartSec=5
PrivateTmp=yes
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cc-switch-server

[Install]
WantedBy=multi-user.target
SERVICEEOF

    systemctl daemon-reload
    systemctl enable cc-switch-server

    ok "Systemd service registered"
}

# ---- Main ----
main() {
    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║   cc-switch-server Installer         ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════╝${NC}"
    echo ""
    log "Prefix:    ${PREFIX}"
    log "Data:      ${DATA_DIR}"
    log "Version:   ${VERSION}"
    echo ""

    # Check if we're in the source directory
    if [[ -f "$(dirname "$0")/go.mod" ]]; then
        log "Detected source tree — will build from source."
        install_local
    else
        install_from_release
    fi

    install_systemd

    # Done
    echo ""
    echo -e "${GREEN}════════════════════════════════════════${NC}"
    echo -e "${GREEN}  Installation complete!${NC}"
    echo -e "${GREEN}════════════════════════════════════════${NC}"
    echo ""
    echo "  Start service:  sudo systemctl start cc-switch-server"
    echo "  Check status:   sudo systemctl status cc-switch-server"
    echo "  View logs:      sudo journalctl -u cc-switch-server -f"
    echo "  Web panel:      http://<server-ip>:9876"
    echo ""
    echo "  CLI usage:"
    echo "    ${PREFIX}/cc-switch list"
    echo "    ${PREFIX}/cc-switch status"
    echo "    ${PREFIX}/cc-switch add --preset deepseek --key <your-key>"
    echo "    ${PREFIX}/cc-switch set deepseek"
    echo ""
}

main "$@"
