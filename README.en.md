<p align="center"><img src="assets/logo.png" width="96" height="96" alt="EasyNodeDeck"></p>

# EasyNodeDeck

[简体中文](README.md) · English

A lightweight, self-hosted panel for running proxy connections on your own Linux server. The local-node release supports Shadowsocks, VLESS + REALITY and Hysteria 2, stable subscription URLs and QR codes.

## Install

Run this command as root on Debian 12 (amd64 or arm64). The installer accepts a public IPv4 address or a domain already pointing to the server. Leave the username and password blank to generate credentials.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/sl2782087/easynodedeck/v0.1.0/install.sh)
```

Make TCP 80 available for certificate validation and renewal and TCP 443 for the panel. The default data ports are 30443 TCP/UDP (SS), 30444 TCP (REALITY) and 30445 UDP (HY2). Allow the protocols you enable through your provider firewall. The installer obtains a trusted certificate and sets up a systemd renewal timer.

## Get a working subscription

1. Open the HTTPS address printed by the installer and sign in. Initial credentials are also saved to `/etc/easynodedeck/initial-access` with root-only permissions.
2. In **Nodes**, choose **Add local node**, name the server and select protocols. Saving starts a dedicated sing-box instance. Repeat saves preserve credentials. Existing services run independently.
3. In **Subscriptions**, choose **Create subscription**, select connections and copy the URL or scan the QR code. Import it into Clash Verge Rev or Shadowrocket. Editing the selection keeps the same URL. Use `?format=clash` or `?format=uri` to override client detection.
4. Open **Account** to change the administrator username or password. Removing a subscription revokes its URL. Clients that already downloaded a connection keep its credentials until the underlying node credentials change.

The service runs as a dedicated system user. Go serves the embedded UI and supervises the local core. Single-administrator state is saved atomically in a private JSON file at `/var/lib/easynodedeck/state.json`. Back it up securely to retain credentials and subscription URLs. Inspect service health with `systemctl status easynodedeck`. Configure protocol ports and the REALITY handshake domain with `SS_PORT`, `REALITY_PORT`, `HY2_PORT` and `REALITY_SNI` in `/etc/easynodedeck/panel.env`, then restart the service.

SSH provisioning for remote machines and importing third-party share links are planned for a later release. For an offline UI demonstration, open [web/index.html](web/index.html) directly as a local file.

![Node management design preview](assets/panel-preview.webp)

## Contributing

We welcome [issues](https://github.com/sl2782087/easynodedeck/issues) in English or Chinese about installation, node management and client compatibility. For compatibility reports, include the operating system, client version and sanitized reproduction steps. Keep SSH credentials, private keys and active subscription URLs private.

Released under the [MIT License](LICENSE).
