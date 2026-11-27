#!/usr/bin/env bash
set -Eeuo pipefail

REPO_URL="${CHECKUSER_REPO_URL:-https://github.com/Lacasot8/Checkuser.git}"
APP_DIR="/usr/local/lib/checkuser"
ETC_DIR="/etc/checkuser"
BIN="$APP_DIR/checkuser"
ENV_FILE="$ETC_DIR/checkuser.env"
SERVICE="checkuser-lacasot8.service"
TUNNEL_SERVICE="checkuser-lacasot8-tunnel.service"

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

if ! command -v curl >/dev/null && ! command -v wget >/dev/null; then
  if command -v apt-get >/dev/null; then apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl; else fail "curl ou wget é necessário para instalar o túnel."; fi
fi
if [[ ! -x /usr/local/bin/cloudflared ]]; then
  echo "Instalando cloudflared..."
  case "$arch" in
    x86_64|amd64) tunnel_arch=amd64 ;;
    aarch64|arm64) tunnel_arch=arm64 ;;
  esac
  tunnel_url="https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-$tunnel_arch"
  if command -v curl >/dev/null; then curl -fL --retry 3 "$tunnel_url" -o /usr/local/bin/cloudflared; else wget -qO /usr/local/bin/cloudflared "$tunnel_url"; fi
  chmod 0755 /usr/local/bin/cloudflared
  /usr/local/bin/cloudflared --version >/dev/null || fail "download do cloudflared falhou."
fi

mkdir -p "$APP_DIR" "$ETC_DIR"
work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
if [[ -n "${CHECKUSER_SOURCE_DIR:-}" && -f "$CHECKUSER_SOURCE_DIR/checkuser.go" ]]; then
  cp "$CHECKUSER_SOURCE_DIR"/* "$work/"
  menu_source="$CHECKUSER_SOURCE_DIR/ck"
else
  git clone --depth 1 "$REPO_URL" "$work/src" >/dev/null 2>&1
  cp "$work/src"/* "$work/"
  menu_source="$work/src/ck"
fi
(cd "$work" && GOOS=linux GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$BIN" .)
chmod 0755 "$BIN"

if [[ ! -f "$ENV_FILE" ]]; then
  cat >"$ENV_FILE" <<'EOF'
CHECKUSER_HOST=0.0.0.0
CHECKUSER_PORT=2053
CHECKUSER_LIMITS_FILE=/root/usuarios.db
CHECKUSER_DRAGONCORE_URL=http://127.0.0.1:9090
CHECKUSER_DRAGONCORE_PATH=/check
CHECKUSER_TIMEOUT_SECONDS=5
EOF
  chmod 0600 "$ENV_FILE"
fi
# Preserve the configured port (default 2053). The ck menu checks availability
# before changing it, so 2052 can be selected when no other service uses it.
APP_PORT="$(awk -F= '$1=="CHECKUSER_PORT"{print $2}' "$ENV_FILE" | tail -1)"
[[ "$APP_PORT" =~ ^[0-9]{1,5}$ ]] && (( APP_PORT >= 1 && APP_PORT <= 65535 )) || fail "CHECKUSER_PORT inválida: $APP_PORT"

cat > "/etc/systemd/system/$SERVICE" <<EOF
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
cat > "/etc/systemd/system/$TUNNEL_SERVICE" <<EOF
[Unit]
Description=Cloudflare quick tunnel for Lacasot8 CheckUser
After=$SERVICE network-online.target
Requires=$SERVICE

[Service]
Type=simple
EnvironmentFile=-$ENV_FILE
ExecStart=/usr/local/bin/cloudflared tunnel --no-autoupdate --url http://127.0.0.1:\${CHECKUSER_PORT}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
install -m 0755 "$menu_source" /usr/local/bin/ck
systemctl daemon-reload
systemctl enable --now "$SERVICE"
systemctl enable --now "$TUNNEL_SERVICE"
echo "CheckUser instalado. Execute: ck"
