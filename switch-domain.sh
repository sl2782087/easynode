# Embedded in the root-owned panel binary. Arguments are validated by domainHelper.
set -Eeuo pipefail
umask 077
host=$1
ipv4=$2
unit=easynodedeck.service
cert=easynodedeck
etc=/etc/easynodedeck
live=/etc/letsencrypt/live/$cert
renew=/etc/letsencrypt/renewal/$cert.conf
exec 9>/run/easynodedeck-acme.lock
flock -n 9 || { echo '证书任务正在运行，请稍后重试 / Another certificate task is running'; exit 1; }
probe() {
  curl --noproxy '*' --max-time 4 --connect-timeout 2 --retry 4 --retry-connrefused --resolve "$host:443:127.0.0.1" -fsS "https://$host/web/" -o /dev/null
}
if [[ $PUBLIC_URL == "https://$host" ]] && probe; then echo current; exit 0; fi
backup=$(mktemp -d "$etc/.switch.XXXXXXXX")
trap 'rm -rf -- "$backup"' EXIT
cp -a "$etc/panel.env" "$renew" "$backup/"
cp -a "$live" "$backup/live"
restore_files() {
  cp -a --remove-destination "$backup/live/"*.pem "$live/"
  cp -a "$backup/$cert.conf" "$renew"
  cp -a "$backup/panel.env" "$etc/panel.env"
}
rollback() {
  trap - ERR
  restore_files
  systemctl restart "$unit"
}
trap rollback ERR
if ! timeout 140 /opt/easynodedeck-certbot/bin/certbot certonly --standalone --cert-name "$cert" --non-interactive --agree-tos --register-unsafely-without-email --force-renewal -d "$host" >&2; then
  restore_files
  echo '证书申请失败，请检查公网 TCP 80、DNS 和签发限制 / Issuance failed. Check public TCP 80, DNS and CA limits'
  exit 1
fi
sed "s|^PUBLIC_URL=.*|PUBLIC_URL=https://$host|; /^PANEL_IPV4=/d" "$etc/panel.env" > "$backup/new.env"
if [[ -n $ipv4 ]]; then printf 'PANEL_IPV4=%s\n' "$ipv4" >> "$backup/new.env"; fi
mv "$backup/new.env" "$etc/panel.env"
# Keep running if the browser leaves after submitting the change.
trap '' PIPE
echo ready || true
read -r -t 5 _ || true
systemctl restart "$unit"
probe
trap - ERR
if [[ -f $etc/initial-access ]]; then sed -i "s|^URL=.*|URL=https://$host|" "$etc/initial-access"; fi
