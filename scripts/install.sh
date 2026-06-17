#!/usr/bin/env bash
# Nexon installer:
#   curl -fsSL https://raw.githubusercontent.com/BX-Team/Nexon/main/scripts/install.sh | sudo bash -s install
# Commands: install | update | uninstall
set -euo pipefail

REPO="BX-Team/Nexon"
BIN="/usr/local/bin/nexon"
ETC="/etc/nexon"
DATA="/var/lib/nexon"
UNIT="/etc/systemd/system/nexon.service"

log()  { printf '\033[1;32m[nexon]\033[0m %s\n' "$*"; }
err()  { printf '\033[1;31m[nexon]\033[0m %s\n' "$*" >&2; exit 1; }

need_root() { [ "$(id -u)" -eq 0 ] || err "run as root (sudo)"; }

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    armv7l|armv7|armhf|arm) echo "arm" ;;
    *) err "unsupported arch: $(uname -m)" ;;
  esac
}

download_binary() {
  local arch tag url
  arch="$(detect_arch)"
  tag="${NEXON_VERSION:-latest}"
  if [ "$tag" = "latest" ]; then
    url="https://github.com/${REPO}/releases/latest/download/nexon-linux-${arch}"
  else
    url="https://github.com/${REPO}/releases/download/${tag}/nexon-linux-${arch}"
  fi
  log "downloading $url"
  curl -fsSL "$url" -o "$BIN" || err "download failed"
  chmod +x "$BIN"
}

write_unit() {
  cat > "$UNIT" <<EOF
[Unit]
Description=Nexon control-plane (subscription server + traffic poller)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment=NEXON_DATA_DIR=${DATA}
EnvironmentFile=-${ETC}/nexon.env
ExecStart=${BIN} serve
Restart=on-failure
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
EOF
}

cmd_install() {
  need_root
  command -v curl >/dev/null || err "curl is required"
  mkdir -p "$ETC" "$DATA"
  [ -f "${ETC}/nexon.env" ] || cat > "${ETC}/nexon.env" <<'EOF'
# Nexon environment. See internal/config/config.go for all keys.
NEXON_SUB_LISTEN=:8080
NEXON_SUB_BASE_URL=https://your.domain
EOF
  download_binary
  write_unit
  systemctl daemon-reload
  systemctl enable --now nexon.service
  log "installed. Edit ${ETC}/nexon.env then: systemctl restart nexon"
}

cmd_update() {
  need_root
  download_binary
  systemctl restart nexon.service
  log "updated to $(${BIN} version)"
}

cmd_uninstall() {
  need_root
  systemctl disable --now nexon.service 2>/dev/null || true
  rm -f "$UNIT" "$BIN"
  systemctl daemon-reload
  log "removed binary + unit. Data kept at ${DATA} (rm -rf to purge)."
}

case "${1:-install}" in
  install)   cmd_install ;;
  update)    cmd_update ;;
  uninstall) cmd_uninstall ;;
  *) err "usage: install.sh [install|update|uninstall]" ;;
esac
