#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $(id -u) == 0 ]] || { echo 'Run as root.' >&2; exit 1; }
supported='Debian 12/13, Ubuntu 22.04/24.04 LTS (amd64/arm64)'
[[ -r /etc/os-release ]] || { echo "Supported: $supported" >&2; exit 78; }
. /etc/os-release
case "${ID:-}:${VERSION_ID:-}" in
  debian:12|debian:13|ubuntu:22.04|ubuntu:24.04) ;;
  *) echo "Unsupported system: ${PRETTY_NAME:-unknown}. Supported: $supported" >&2; exit 78 ;;
esac
[[ -d /run/systemd/system ]] && command -v systemctl >/dev/null || { echo 'A running systemd host is required.' >&2; exit 78; }
case "$(uname -m)" in
  x86_64) arch=amd64; core_sha=bb99cabf47694625db421ee17898f36cdc1f9c2cb5decf65b12bac8d8437e842 ;;
  aarch64) arch=arm64; core_sha=d7fab87b921933eb281d8ee7bd5377cdd8228089f1f7c807c9363a6a2329286c ;;
  *) echo 'Supported architectures: amd64, arm64' >&2; exit 78 ;;
esac
# The panel also runs this preflight on remote nodes before creating their identity.
packages=(ca-certificates curl tar gzip openssl util-linux passwd)
case "${1:-}" in
  '') packages+=(iproute2 python3-venv) ;;
  --prepare-node-tls) packages+=(python3-venv) ;;
  --prepare-node) ;;
  *) echo "Unknown argument: $1" >&2; exit 2 ;;
esac
missing=()
for package in "${packages[@]}"; do
  if [[ $(dpkg-query -W -f='${db:Status-Status}' "$package" 2>/dev/null) != installed ]]; then
    missing+=("$package")
  fi
done
if (( ${#missing[@]} )); then
  (umask 022
   export DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=l
   apt-get -o Acquire::Retries=2 -qq update
   apt-get -o Acquire::Retries=2 -qq install -y --no-install-recommends "${missing[@]}")
fi
[[ -z ${1:-} ]] || exit 0
root=/opt/easynodedeck
etc=/etc/easynodedeck
state=/var/lib/easynodedeck
acme=/opt/easynodedeck-certbot
scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT
install -d -m 0755 "$root"
install -d -m 0700 "$etc"
if [[ -f "$etc/panel.env" ]]; then
  public_url=$(sed -n 's/^PUBLIC_URL=//p' "$etc/panel.env")
  host=${public_url#https://}
else
  default_ip=$(ip -4 route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++)if($i=="src")print $(i+1)}')
  host=${PUBLIC_HOST:-}
  if [[ -z $host && -t 0 ]]; then read -rp "域名或公网 IP / Public host [$default_ip]: " host; fi
  host=${host:-$default_ip}
fi
[[ $host =~ ^[A-Za-z0-9][A-Za-z0-9.-]*$ ]] || { echo 'Enter a public IPv4 address or DNS name.' >&2; exit 1; }
user= password=
if [[ ! -f "$state/state.json" ]]; then
  if [[ -t 0 ]]; then
    read -rp '管理员用户名 / Username [generate]: ' user
    read -rsp '管理员密码 / Password [generate]: ' password
    echo
  fi
  user=${user:-admin-$(openssl rand -hex 3)}
  password=${password:-$(openssl rand -hex 18)}
  [[ ${#password} -ge 8 && ${#password} -le 1024 ]] || { echo 'Password must contain 8–1024 bytes.' >&2; exit 1; }
fi
release=https://github.com/sl2782087/easynodedeck/releases/download/v0.3.0
if [[ -z ${DECK_BINARY:-} ]]; then
  asset=easynodedeck-linux-$arch
  curl -fsSL --retry 2 "$release/$asset" -o "$scratch/$asset"
  curl -fsSL --retry 2 "$release/SHA256SUMS" -o "$scratch/sums"
  (cd "$scratch"; awk -v n="$asset" '$2==n' sums | sha256sum -c -)
  DECK_BINARY=$scratch/$asset
fi
if command -v sing-box >/dev/null && [[ $(sing-box version | head -1) == "sing-box version 1.13."* ]]; then
  core=$(command -v sing-box)
else
  base=sing-box-1.13.13-linux-$arch
  curl -fsSL --retry 2 "https://github.com/SagerNet/sing-box/releases/download/v1.13.13/$base.tar.gz" -o "$scratch/core.tgz"
  echo "$core_sha  $scratch/core.tgz" | sha256sum -c -
  tar -xzf "$scratch/core.tgz" -C "$scratch" "$base/sing-box"
  install -m 0755 "$scratch/$base/sing-box" "$root/sing-box.new"
  mv "$root/sing-box.new" "$root/sing-box"
  core=$root/sing-box
fi
if [[ ! -x "$acme/bin/certbot" ]]; then
  python3 -m venv "$acme"
  "$acme/bin/pip" -q install 'certbot>=5.4,<7'
fi
if [[ $host =~ ^[0-9.]+$ ]]; then
  cert_args=(--ip-address "$host" --preferred-profile shortlived)
else
  cert_args=(-d "$host")
fi
"$acme/bin/certbot" certonly --standalone --cert-name easynodedeck --non-interactive --agree-tos \
  --register-unsafely-without-email --keep-until-expiring "${cert_args[@]}"

install -m 0755 "$DECK_BINARY" "$root/panel.new"
mv "$root/panel.new" "$root/panel"
id easynodedeck >/dev/null 2>&1 || useradd --system --user-group --no-create-home --home-dir "$state" --shell /usr/sbin/nologin easynodedeck
install -d -m 0700 -o easynodedeck -g easynodedeck "$state"
if [[ ! -f "$state/state.json" ]]; then
  printf '%s' "$password" | runuser -u easynodedeck -- env STATE_DIRECTORY="$state" ADMIN_USER="$user" "$root/panel" init
  printf 'URL=https://%s\nUsername=%s\nPassword=%s\n' "$host" "$user" "$password" > "$etc/initial-access"
fi
if [[ ! -f "$etc/panel.env" ]]; then
  printf 'PUBLIC_URL=https://%s\nCORE_PATH=%s\n' "$host" "$core" > "$etc/panel.env"
fi
cat > /etc/systemd/system/easynodedeck.service <<'UNIT'
[Unit]
Description=EasyNodeDeck local node and subscription panel
After=network-online.target
Wants=network-online.target
[Service]
User=easynodedeck
Group=easynodedeck
EnvironmentFile=/etc/easynodedeck/panel.env
StateDirectory=easynodedeck
StateDirectoryMode=0700
WorkingDirectory=/var/lib/easynodedeck
LoadCredential=cert.pem:/etc/letsencrypt/live/easynodedeck/fullchain.pem
LoadCredential=key.pem:/etc/letsencrypt/live/easynodedeck/privkey.pem
ExecStart=/opt/easynodedeck/panel
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
UMask=0077
Restart=on-failure
RestartSec=3
[Install]
WantedBy=multi-user.target
UNIT
cat > /etc/systemd/system/easynodedeck-renew.service <<'UNIT'
[Unit]
Description=Renew EasyNodeDeck TLS certificate
[Service]
Type=oneshot
ExecStart=/opt/easynodedeck-certbot/bin/certbot renew --cert-name easynodedeck --quiet --deploy-hook "/usr/bin/systemctl restart easynodedeck.service"
UNIT
cat > /etc/systemd/system/easynodedeck-renew.timer <<'UNIT'
[Unit]
Description=Check EasyNodeDeck certificate twice daily
[Timer]
OnCalendar=*-*-* 00,12:00:00
RandomizedDelaySec=45min
Persistent=true
[Install]
WantedBy=timers.target
UNIT
chmod 0644 /etc/systemd/system/easynodedeck{.service,-renew.service,-renew.timer}
systemctl daemon-reload
systemctl enable easynodedeck.service easynodedeck-renew.timer
systemctl start easynodedeck-renew.timer
systemctl restart easynodedeck.service
curl --noproxy '*' --resolve "$host:443:127.0.0.1" -fsS --retry 5 --retry-connrefused "https://$host/web/" -o /dev/null
printf 'EasyNodeDeck: https://%s/\n' "$host"
if [[ -n $user ]]; then cat "$etc/initial-access"; fi
