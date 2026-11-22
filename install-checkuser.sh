#!/usr/bin/env bash
set -Eeuo pipefail

REPO_URL="${CHECKUSER_REPO_URL:-https://github.com/Lacasot8/Checkuser.git}"
APP_DIR="/usr/local/lib/checkuser"
ETC_DIR="/etc/checkuser"
BIN="$APP_DIR/checkuser"
ENV_FILE="$ETC_DIR/checkuser.env"
SERVICE="checkuser.service"
TUNNEL_SERVICE="checkuser-tunnel.service"

fail(){ echo "Erro: $*" >&2; exit 1; }
[[ $EUID -eq 0 ]] || fail "execute como root (sudo -i)."
command -v systemctl >/dev/null || fail "systemd não foi encontrado."
command -v git >/dev/null || fail "git é necessário para instalar."
if ! command -v go >/dev/null; then
  if command -v apt-get >/dev/null; then
    echo "Go não encontrado; instalando golang-go pelo apt..."
    apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq golang-go
  else
    fail "Go não encontrado e o sistema não possui apt-get. Instale Go 1.21+ e execute novamente."
  fi
fi

arch="$(uname -m)"
case "$arch" in x86_64|amd64) goarch=amd64 ;; aarch64|arm64) goarch=arm64 ;; *) fail "arquitetura não suportada: $arch (use amd64 ou arm64)." ;; esac
mkdir -p "$APP_DIR" "$ETC_DIR"
work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
if [[ -n "${CHECKUSER_SOURCE_DIR:-}" && -f "$CHECKUSER_SOURCE_DIR/checkuser.go" ]]; then
  cp "$CHECKUSER_SOURCE_DIR"/* "$work/"
  menu_source="$CHECKUSER_SOURCE_DIR/check"
else
  git clone --depth 1 "$REPO_URL" "$work/src" >/dev/null 2>&1
  cp "$work/src"/* "$work/"
  menu_source="$work/src/check"
fi
(cd "$work" && GOOS=linux GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$BIN" .)
chmod 0755 "$BIN"

if [[ ! -f "$ENV_FILE" ]]; then
  cat >"$ENV_FILE" <<'EOF'
CHECKUSER_HOST=0.0.0.0
CHECKUSER_PORT=2052
CHECKUSER_LIMITS_FILE=/root/usuarios.db
CHECKUSER_DRAGONCORE_URL=http://127.0.0.1:9090
CHECKUSER_DRAGONCORE_PATH=/check
CHECKUSER_TIMEOUT_SECONDS=5
EOF
  chmod 0600 "$ENV_FILE"
fi
cat > /etc/systemd/system/$SERVICE <<EOF
[Unit]
Description=CheckUser HTTP service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$BIN
EnvironmentFile=-$ENV_FILE
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=false
ReadWritePaths=/root

[Install]
WantedBy=multi-user.target
EOF
cat > /etc/systemd/system/$TUNNEL_SERVICE <<'EOF'
[Unit]
Description=Cloudflare quick tunnel for CheckUser
After=checkuser.service network-online.target
Requires=checkuser.service

[Service]
Type=simple
ExecStart=/usr/local/bin/cloudflared tunnel --no-autoupdate --url http://127.0.0.1:2052
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
install -m 0755 "$menu_source" /usr/local/bin/check
systemctl daemon-reload
systemctl enable --now "$SERVICE"
echo "CheckUser instalado. Execute: check"
