#!/usr/bin/env bash
set -Eeuo pipefail
SERVICE=checkuser-lacasot8.service
TUNNEL=checkuser-lacasot8-tunnel.service
ENV_FILE=/etc/checkuser/checkuser.env
PORT=2053
[[ -f "$ENV_FILE" ]] && PORT="$(awk -F= '$1=="CHECKUSER_PORT"{print $2}' "$ENV_FILE" | tail -1)"
FRAME_WIDTH=56
TEXT_WIDTH=54

need_root(){ [[ $EUID -eq 0 ]] || { echo 'Execute como root.'; exit 1; }; }
active(){ systemctl is-active --quiet "$1" 2>/dev/null; }
state(){ active "$1" && printf 'ativo' || printf 'inativo'; }
link(){ journalctl -u "$TUNNEL" --no-pager -n 100 2>/dev/null | grep -Eo 'https://[a-z0-9-]+\.trycloudflare\.com' | tail -1 || true; }
border(){ printf '\033[91m╭'; printf '─%.0s' $(seq 1 "$FRAME_WIDTH"); printf '╮\033[0m\n'; }
separator(){ printf '\033[91m├'; printf '─%.0s' $(seq 1 "$FRAME_WIDTH"); printf '┤\033[0m\n'; }
row(){ printf '\033[91m│\033[0m %-*.*s \033[91m│\033[0m\n' "$TEXT_WIDTH" "$TEXT_WIDTH" "$1"; }

show(){
 clear
 border
 row 'CHECKUSER LACASOT8'
 row 'Painel de controle'
 separator
 row "CheckUser:  $(state "$SERVICE")"
 row "Cloudflare: $(state "$TUNNEL")"
 row "Porta:      $PORT"
 separator
 row 'LINK / DTUNNEL'
 local url; url="$(link)"
 if [[ -n "$url" ]]; then
  while IFS= read -r part; do row "$part"; done < <(printf '%s' "$url" | fold -w "$TEXT_WIDTH")
 else
  row 'Link ainda não gerado'
 fi
 while IFS= read -r part; do row "$part"; done < <(printf '%s' '/check/{username}?deviceId={deviceId}' | fold -w "$TEXT_WIDTH")
 row "Porta local: http://127.0.0.1:$PORT"
 row "Logs: journalctl -u $TUNNEL"
 border
 row '[1] Iniciar serviços'
 row '[2] Parar serviços'
 row '[3] Reiniciar serviços e túnel'
 row '[4] Status detalhado'
 row '[5] Ver logs do Cloudflare'
 row '[6] Atualizar link/tela'
 row '[7] Remover somente CheckUser Lacasot8'
 row '[8] Alterar porta'
 row '[0] Sair'
 border
}
wait_key(){ read -r -p 'ENTER para continuar...' _; }
change_port(){
 local new_port tmp backup
 echo
 read -r -p "Nova porta TCP (atual: $PORT): " new_port
 if [[ ! "$new_port" =~ ^[0-9]{1,5}$ ]] || (( 10#$new_port < 1 || 10#$new_port > 65535 )); then
  echo 'Porta inválida. Informe um número de 1 a 65535.'; wait_key; return 1
 fi
 new_port=$((10#$new_port))
 if [[ "$new_port" == "$PORT" ]]; then echo "A porta já está configurada como $PORT."; wait_key; return 0; fi
 if ! command -v ss >/dev/null 2>&1; then echo 'Não encontrei o comando ss; não alterei a configuração.'; wait_key; return 1; fi
 if [[ -n "$(ss -Hln "sport = :$new_port" 2>/dev/null)" ]]; then
  echo "A porta $new_port já está em uso. Nada foi alterado; escolha outra porta."
  wait_key; return 1
 fi
 [[ -f "$ENV_FILE" ]] || { echo "Configuração não encontrada: $ENV_FILE"; wait_key; return 1; }
 tmp="$(mktemp "${ENV_FILE}.XXXXXX")"
 backup="$(mktemp)"
 cp -p "$ENV_FILE" "$backup"
 awk -F= -v port="$new_port" 'BEGIN{done=0} /^CHECKUSER_PORT=/ {if(!done) print "CHECKUSER_PORT=" port; done=1; next} {print} END{if(!done) print "CHECKUSER_PORT=" port}' "$ENV_FILE" > "$tmp"
 chmod --reference="$ENV_FILE" "$tmp"
 chown --reference="$ENV_FILE" "$tmp" 2>/dev/null || true
 mv "$tmp" "$ENV_FILE"
 if systemctl restart "$SERVICE" && systemctl restart "$TUNNEL"; then
  rm -f "$backup"
  PORT="$new_port"
  echo "Porta alterada para $PORT; serviço e túnel reiniciados."
  wait_key
  return 0
 fi
 cp -p "$backup" "$ENV_FILE"
 rm -f "$backup"
 systemctl restart "$SERVICE" >/dev/null 2>&1 || true
 systemctl restart "$TUNNEL" >/dev/null 2>&1 || true
 echo 'Falha ao reiniciar. Restaurei a configuração anterior; confira o status dos serviços.'
 wait_key
 return 1
}
remove(){
 echo; read -r -p 'Digite REMOVER para confirmar: ' answer
 [[ "$answer" == REMOVER ]] || { echo 'Cancelado.'; wait_key; return; }
 systemctl disable --now "$TUNNEL" "$SERVICE" 2>/dev/null || true
 rm -f "/etc/systemd/system/$TUNNEL" "/etc/systemd/system/$SERVICE" /usr/local/bin/ck
 rm -rf /usr/local/lib/checkuser /etc/checkuser
 systemctl daemon-reload
 echo 'Checkuser Lacasot8 removido. /root/usuarios.db, cloudflared e o comando check foram preservados.'
 exit 0
}
start_all(){ systemctl start "$SERVICE" && systemctl restart "$TUNNEL"; }
need_root
while true; do
 show
 read -r -p 'Opção: ' op
 case "$op" in
  1) start_all; wait_key ;;
  2) systemctl stop "$TUNNEL" "$SERVICE"; wait_key ;;
  3) systemctl restart "$SERVICE" && systemctl restart "$TUNNEL"; sleep 2; wait_key ;;
  4) systemctl --no-pager --full status "$SERVICE" "$TUNNEL"; wait_key ;;
  5) journalctl -u "$TUNNEL" -n 100 --no-pager; wait_key ;;
  6) systemctl restart "$TUNNEL"; sleep 2; wait_key ;;
  7) remove ;;
  8) change_port || true ;;
  0|q|Q) exit 0 ;;
  *) echo 'Opção inválida.'; sleep 1 ;;
 esac
done
