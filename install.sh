#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $(id -u) == 0 ]] || { echo 'Run as root.' >&2; exit 1; }
supported='Debian 12/13, Ubuntu 22.04/24.04/26.04 LTS, Rocky Linux 8/9/10, CentOS Stream 9/10 (amd64/arm64)'
[[ -r /etc/os-release ]] || { echo "Supported: $supported" >&2; exit 78; }
. /etc/os-release
major=${VERSION_ID:-}
major=${major%%.*}
python=python3
case "${ID:-}:${VERSION_ID:-}" in
  debian:12|debian:13|ubuntu:22.04|ubuntu:24.04|ubuntu:26.04) manager=apt ;;
  rocky:8|rocky:8.*|rocky:9|rocky:9.*|rocky:10|rocky:10.*) manager=dnf ;;
  centos:9|centos:10)
    [[ ${NAME:-} == "CentOS Stream" ]] || { echo "Supported: $supported" >&2; exit 78; }
    manager=dnf ;;
  *) echo "Unsupported system: ${PRETTY_NAME:-unknown}. Supported: $supported" >&2; exit 78 ;;
esac
[[ -d /run/systemd/system ]] && command -v systemctl >/dev/null || { echo 'A running systemd host is required.' >&2; exit 78; }
case "$(uname -m)" in
  x86_64) arch=amd64; core_sha=bb99cabf47694625db421ee17898f36cdc1f9c2cb5decf65b12bac8d8437e842 ;;
  aarch64) arch=arm64; core_sha=d7fab87b921933eb281d8ee7bd5377cdd8228089f1f7c807c9363a6a2329286c ;;
  *) echo 'Supported architectures: amd64, arm64' >&2; exit 78 ;;
esac
# The panel also runs this preflight on remote nodes before creating their identity.
role=${1:-panel}
case "$role" in panel|--prepare-node|--prepare-node-tls) ;; *) echo "Unknown role: $role" >&2; exit 2 ;; esac
packages=(ca-certificates tar gzip openssl util-linux)
if [[ $manager == apt ]]; then
  packages+=(curl passwd)
  [[ $role != panel ]] || packages+=(iproute2)
  [[ $role == --prepare-node ]] || packages+=(python3-venv)
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
else
  packages+=(shadow-utils)
  command -v curl >/dev/null || packages+=(curl)
  [[ $role != panel ]] || packages+=(iproute)
  if [[ $role != --prepare-node ]]; then
    if [[ $major == 10 ]]; then packages+=(python3 python3-pip)
    else python=python3.11; packages+=(python3.11 python3.11-pip); fi
  fi
  if ! rpm -q "${packages[@]}" >/dev/null; then
    (umask 022; dnf -y --setopt=install_weak_deps=False install "${packages[@]}")
  fi
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
release=https://github.com/sl2782087/easynode/releases/download/v0.6.2
if [[ -z ${DECK_BINARY:-} ]]; then
  asset=easynode-linux-$arch
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
  "$python" -m venv "$acme"
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
Description=EasyNode local node and subscription panel
After=network-online.target
Wants=network-online.target
[Service]
User=easynodedeck
Group=easynodedeck
EnvironmentFile=/etc/easynodedeck/panel.env
StateDirectory=easynodedeck
StateDirectoryMode=0700
WorkingDirectory=/var/lib/easynodedeck
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
unit=/etc/systemd/system/easynodedeck.service
if [[ $ID == rocky && $major == 8 ]]; then
  # systemd 239 predates LoadCredential. Only root can write this directory.
  cat >> "$unit" <<'UNIT'
[Service]
Environment=CREDENTIALS_DIRECTORY=/run/easynodedeck-certs
ExecStartPre=+/usr/bin/install -d -m 0750 -o root -g easynodedeck /run/easynodedeck-certs
ExecStartPre=+/usr/bin/install -m 0440 -o root -g easynodedeck /etc/letsencrypt/live/easynodedeck/fullchain.pem /run/easynodedeck-certs/cert.pem
ExecStartPre=+/usr/bin/install -m 0440 -o root -g easynodedeck /etc/letsencrypt/live/easynodedeck/privkey.pem /run/easynodedeck-certs/key.pem
UNIT
else
  cat >> "$unit" <<'UNIT'
[Service]
LoadCredential=cert.pem:/etc/letsencrypt/live/easynodedeck/fullchain.pem
LoadCredential=key.pem:/etc/letsencrypt/live/easynodedeck/privkey.pem
UNIT
fi
cat > /etc/systemd/system/easynodedeck-renew.service <<'UNIT'
[Unit]
Description=Renew EasyNode TLS certificate
[Service]
Type=oneshot
ExecStart=/usr/bin/flock /run/easynodedeck-acme.lock /opt/easynodedeck-certbot/bin/certbot renew --cert-name easynodedeck --quiet --deploy-hook "/usr/bin/systemctl restart easynodedeck.service"
UNIT
cat > /etc/systemd/system/easynodedeck-renew.timer <<'UNIT'
[Unit]
Description=Check EasyNode certificate twice daily
[Timer]
OnCalendar=*-*-* 00,12:00:00
RandomizedDelaySec=45min
Persistent=true
[Install]
WantedBy=timers.target
UNIT
cat > /etc/systemd/system/easynodedeck-domain.socket <<'UNIT'
[Unit]
Description=EasyNode domain configuration socket
[Socket]
ListenStream=/run/easynodedeck-domain.sock
SocketMode=0660
SocketGroup=easynodedeck
Accept=yes
MaxConnections=2
[Install]
WantedBy=sockets.target
UNIT
cat > /etc/systemd/system/easynodedeck-domain@.service <<'UNIT'
[Unit]
Description=Configure the EasyNode panel domain
[Service]
EnvironmentFile=/etc/easynodedeck/panel.env
ExecStart=/opt/easynodedeck/panel domain-helper
StandardInput=socket
StandardOutput=inherit
StandardError=journal
UMask=0077
RuntimeMaxSec=240
UNIT
chmod 0644 /etc/systemd/system/easynodedeck-domain{.socket,@.service}
chmod 0644 /etc/systemd/system/easynodedeck{.service,-renew.service,-renew.timer}
systemctl daemon-reload
systemctl enable easynodedeck.service easynodedeck-renew.timer
systemctl enable --now easynodedeck-domain.socket
systemctl start easynodedeck-renew.timer
systemctl restart easynodedeck.service
curl --noproxy '*' --resolve "$host:443:127.0.0.1" -fsS --retry 5 --retry-connrefused "https://$host/web/" -o /dev/null
printf 'EasyNode: https://%s/\n' "$host"
if [[ -n $user ]]; then cat "$etc/initial-access"; fi
