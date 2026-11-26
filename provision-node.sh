# Executed by the panel over an authenticated SSH connection.
set -euo pipefail
umask 077
exec >>/var/log/easynodedeck-node-install.log 2>&1
echo "EasyNodeDeck node installation: $(date -u +%FT%TZ)"
root=/opt/easynodedeck-node
etc=/etc/easynodedeck-node
unit=/etc/systemd/system/easynodedeck-node.service
exec 9>"$etc/.lock"
flock -w 30 9
scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT
printf '%s' "$NODE_CONFIG" | base64 -d > "$scratch/node.json"
if cmp -s "$scratch/node.json" "$etc/node.json" && systemctl is-active --quiet easynodedeck-node && systemctl is-enabled --quiet easynodedeck-node; then
  if [[ $WITH_TLS == false ]] || { systemctl is-active --quiet easynodedeck-node-renew.timer && openssl x509 -checkend 43200 -noout -in "$etc/acme/live/node/fullchain.pem"; }; then
    echo "Existing node configuration active; skipped installation"
    exit 0
  fi
fi
install -d -m 0755 "$root"
core="$root/sing-box"
if command -v sing-box >/dev/null && [[ $(sing-box version | head -1) == "sing-box version 1.13."* ]]; then
  core=$(command -v sing-box)
elif [[ ! -x $core ]]; then
  case "$(uname -m)" in
    x86_64) arch=amd64; sha=bb99cabf47694625db421ee17898f36cdc1f9c2cb5decf65b12bac8d8437e842 ;;
    aarch64) arch=arm64; sha=d7fab87b921933eb281d8ee7bd5377cdd8228089f1f7c807c9363a6a2329286c ;;
    *) echo 'Unsupported architecture'; exit 1 ;;
  esac
  base=sing-box-1.13.13-linux-$arch
  curl -fsSL --retry 2 --max-time 150 "https://github.com/SagerNet/sing-box/releases/download/v1.13.13/$base.tar.gz" -o "$scratch/core.tgz"
  echo "$sha  $scratch/core.tgz" | sha256sum -c -
  tar -xzf "$scratch/core.tgz" -C "$scratch" "$base/sing-box"
  install -m 0755 "$scratch/$base/sing-box" "$core.new"
  mv "$core.new" "$core"
fi
id easynodedeck-node >/dev/null 2>&1 || useradd --system --user-group --no-create-home --shell /usr/sbin/nologin easynodedeck-node
certdir=$etc/acme/live/node
if [[ $WITH_TLS == true ]]; then
  acme=$root/acme
  if [[ ! -x "$acme/bin/certbot" ]]; then
    . /etc/os-release
    python=python3
    case "$ID:${VERSION_ID%%.*}" in rocky:8|rocky:9|centos:9) python=python3.11 ;; esac
    "$python" -m venv "$acme"
    "$acme/bin/pip" -q install 'certbot>=5.4,<7'
  fi
  "$acme/bin/certbot" certonly --config-dir "$etc/acme" --work-dir "$etc/acme-work" --logs-dir "$etc/acme-logs" \
    --standalone --cert-name node --ip-address "$NODE_HOST" --preferred-profile shortlived \
    --non-interactive --agree-tos --register-unsafely-without-email --keep-until-expiring
  cat > /etc/systemd/system/easynodedeck-node-renew.service <<UNIT
[Unit]
Description=Renew EasyNodeDeck remote node certificate
[Service]
Type=oneshot
ExecStart=$acme/bin/certbot renew --config-dir $etc/acme --work-dir $etc/acme-work --logs-dir $etc/acme-logs --cert-name node --quiet --deploy-hook "/usr/bin/systemctl restart easynodedeck-node.service"
UNIT
  cat > /etc/systemd/system/easynodedeck-node-renew.timer <<'UNIT'
[Unit]
Description=Check EasyNodeDeck remote node certificate twice daily
[Timer]
OnCalendar=*-*-* 00,12:00:00
RandomizedDelaySec=45min
Persistent=true
[Install]
WantedBy=timers.target
UNIT
  chmod 0644 /etc/systemd/system/easynodedeck-node-renew.{timer,service}
fi
# Check with the real certificate paths outside systemd's credentials namespace.
sed -e "s|/run/credentials/easynodedeck-node.service/cert.pem|$certdir/fullchain.pem|g" \
    -e "s|/run/credentials/easynodedeck-node.service/key.pem|$certdir/privkey.pem|g" "$scratch/node.json" > "$scratch/check.json"
"$core" check -c "$scratch/check.json"
cat > "$scratch/unit" <<UNIT
[Unit]
Description=EasyNodeDeck remote proxy node
After=network-online.target
[Service]
User=easynodedeck-node
Group=easynodedeck-node
ExecStart=$core run -c /run/credentials/easynodedeck-node.service/node.json
Restart=on-failure
RestartSec=3
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
UNIT
if [[ ${LOW_PORT:-false} == true ]]; then
  echo 'AmbientCapabilities=CAP_NET_BIND_SERVICE' >> "$scratch/unit"
fi
# Use the same runtime paths on both generations of systemd.
. /etc/os-release
runtime=/run/credentials/easynodedeck-node.service
if [[ $ID == rocky && ${VERSION_ID%%.*} == 8 ]]; then
  cat >> "$scratch/unit" <<UNIT
ExecStartPre=+/usr/bin/install -d -m 0750 -o root -g easynodedeck-node $runtime
ExecStartPre=+/usr/bin/install -m 0440 -o root -g easynodedeck-node $etc/node.json $runtime/node.json
UNIT
  if [[ $WITH_TLS == true ]]; then
    cat >> "$scratch/unit" <<UNIT
ExecStartPre=+/usr/bin/install -m 0440 -o root -g easynodedeck-node $certdir/fullchain.pem $runtime/cert.pem
ExecStartPre=+/usr/bin/install -m 0440 -o root -g easynodedeck-node $certdir/privkey.pem $runtime/key.pem
UNIT
  fi
else
  echo "LoadCredential=node.json:$etc/node.json" >> "$scratch/unit"
  if [[ $WITH_TLS == true ]]; then
    printf 'LoadCredential=cert.pem:%s/fullchain.pem\nLoadCredential=key.pem:%s/privkey.pem\n' "$certdir" "$certdir" >> "$scratch/unit"
  fi
fi
printf '[Install]\nWantedBy=multi-user.target\n' >> "$scratch/unit"
if ! cmp -s "$scratch/unit" "$unit" || ! cmp -s "$scratch/node.json" "$etc/node.json" || ! systemctl is-active --quiet easynodedeck-node; then
  if [[ -f "$unit" && -f "$etc/node.json" ]]; then cp "$unit" "$scratch/old-unit"; cp "$etc/node.json" "$scratch/old-config"; fi
  install -m 0600 "$scratch/node.json" "$etc/node.new" && mv "$etc/node.new" "$etc/node.json"
  install -m 0644 "$scratch/unit" "$unit.new" && mv "$unit.new" "$unit"
  systemctl daemon-reload
  systemctl enable easynodedeck-node
  if ! (systemctl restart easynodedeck-node && sleep 1 && systemctl is-active --quiet easynodedeck-node); then
    if [[ -f "$scratch/old-unit" ]]; then
      install -m 0644 "$scratch/old-unit" "$unit"; install -m 0600 "$scratch/old-config" "$etc/node.json"
      systemctl daemon-reload && systemctl restart easynodedeck-node
    else systemctl disable --now easynodedeck-node; fi
    exit 1
  fi
fi
if [[ $WITH_TLS == true ]]; then
  systemctl daemon-reload
  systemctl enable --now easynodedeck-node-renew.timer
elif [[ -f /etc/systemd/system/easynodedeck-node-renew.timer ]]; then
  systemctl disable --now easynodedeck-node-renew.timer
fi
echo "Node service active"
