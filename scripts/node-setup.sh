#!/usr/bin/env bash
# Turn a fresh VPS into a Nexon node: install xray-core and expose its gRPC API
# (HandlerService + StatsService) so the Nexon control-plane can sync users and
# poll traffic over it.
#
# The Xray gRPC API has NO authentication of its own — anyone who can reach the
# port controls the node. You MUST restrict it to the Nexon host (firewall
# allow-list or a private network / WireGuard / SSH tunnel). See the README.
#
#   API_PORT=8443 bash node-setup.sh
set -euo pipefail

API_PORT="${API_PORT:-8443}"
# Bind address for the API inbound. Default is all interfaces; set to a private
# IP (e.g. a WireGuard address) to avoid exposing it publicly.
API_LISTEN="${API_LISTEN:-0.0.0.0}"
XRAY_CONF="/usr/local/etc/xray/config.json"

log() { printf '\033[1;32m[node]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[node]\033[0m %s\n' "$*"; }
err() { printf '\033[1;31m[node]\033[0m %s\n' "$*" >&2; exit 1; }
[ "$(id -u)" -eq 0 ] || err "run as root"

log "installing xray-core"
bash -c "$(curl -fsSL https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install

if [ -s "$XRAY_CONF" ]; then
  backup="${XRAY_CONF}.bak.$(date +%Y%m%d-%H%M%S)"
  cp -a "$XRAY_CONF" "$backup"
  warn "existing config backed up to ${backup} — merge your inbounds back manually"
fi

log "writing Xray API inbound to ${XRAY_CONF} (api on ${API_LISTEN}:${API_PORT})"
cat > "$XRAY_CONF" <<EOF
{
  "log": { "loglevel": "warning" },
  "api": { "tag": "api", "services": ["HandlerService", "StatsService"] },
  "stats": {},
  "policy": { "levels": { "0": { "statsUserUplink": true, "statsUserDownlink": true } } },
  "inbounds": [
    {
      "tag": "api",
      "listen": "${API_LISTEN}",
      "port": ${API_PORT},
      "protocol": "dokodemo-door",
      "settings": { "address": "127.0.0.1" }
    }
  ],
  "outbounds": [ { "protocol": "freedom" } ],
  "routing": { "rules": [ { "inboundTag": ["api"], "outboundTag": "api", "type": "field" } ] }
}
EOF

systemctl restart xray
log "done. Add your real proxy inbounds (vless/trojan/hysteria2…) to ${XRAY_CONF}"
log "alongside the api inbound, then register the node on the Nexon host:"
log ""
log "  nexon node add <name> --address <THIS_HOST> --api-port ${API_PORT}"
log "  nexon node inbound add <name> --tag <inbound-tag> --protocol vless --port 443 ..."
log ""
warn "SECURITY: port ${API_PORT} has no auth. Allow it ONLY from the Nexon host"
warn "(e.g. 'ufw allow from <NEXON_IP> to any port ${API_PORT}') or keep it on a private network."
