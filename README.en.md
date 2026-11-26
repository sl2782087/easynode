<p align="center"><img src="assets/logo.png" width="96" height="96" alt="EasyNodeDeck"></p>

# EasyNodeDeck

[简体中文](README.md) · English

A lightweight, self-hosted panel for local and remote Linux nodes. Combine Shadowsocks, VLESS + REALITY and Hysteria 2 connections in stable subscription URLs and QR codes.

## Install

Run this command as root on Debian 12 (amd64 or arm64). The installer accepts a public IPv4 address or a domain already pointing to the server. Leave the username and password blank to generate credentials.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/sl2782087/easynodedeck/v0.2.0/install.sh)
```

Make TCP 80 available for certificate validation and renewal and TCP 443 for the panel. The default data ports are 30443 TCP/UDP (SS), 30444 TCP (REALITY) and 30445 UDP (HY2). Allow the protocols you enable through your provider firewall. The installer obtains a trusted certificate and sets up a systemd renewal timer.

## Get a working subscription

1. Open the HTTPS address printed by the installer and sign in. Initial credentials are also saved to `/etc/easynodedeck/initial-access` with root-only permissions.
2. In **Nodes**, choose **Add node**, select the panel host or a remote server, name it and select protocols. The panel labels local nodes. Subscription clients also show the local marker (本机) in connection names.
3. In **Subscriptions**, choose **Create subscription**, select connections and copy the URL or scan the QR code. Import it into Clash Verge Rev or Shadowrocket. Editing the selection keeps the same URL. Use `?format=clash` or `?format=uri` to override client detection.
4. Open **Account** to change the administrator username or password. Removing a subscription revokes its URL. Clients that already downloaded a connection keep its credentials until the underlying node credentials change.

The service runs as a dedicated system user. Go serves the embedded UI and supervises the local core. Single-administrator state is saved atomically in a private JSON file at `/var/lib/easynodedeck/state.json`. Back it up securely to retain credentials and subscription URLs. Inspect service health with `systemctl status easynodedeck`. Configure protocol ports and the REALITY handshake domain with `SS_PORT`, `REALITY_PORT`, `HY2_PORT` and `REALITY_SNI` in `/etc/easynodedeck/panel.env`, then restart the service.

### Remote VPS deployment

Enter a public IPv4 address, SSH port and root password. Verify the SSH host fingerprint and choose **Deploy and save**. The target needs systemd and an amd64 or arm64 CPU. HY2 uses a free, publicly reachable TCP port 80 for IP certificate issuance and renewal. Connections use the protocol ports listed above. The remote host runs a dedicated `easynodedeck-node.service`.

Repeating deployment reuses the node identity and matching configuration. The SSH password is used for the current operation only. Removing a remote node requires SSH authentication, stops its service and removes its connections from subscriptions. See `/var/log/easynodedeck-node-install.log` on the remote host for installation errors. Reachability in the node list reflects a check of the selected TCP listeners. UDP-only nodes use the last service check over SSH.

For an offline UI demonstration, open [web/index.html](web/index.html) directly as a local file.

![Node management design preview](assets/panel-preview.webp)

## Contributing

We welcome [issues](https://github.com/sl2782087/easynodedeck/issues) in English or Chinese about installation, node management and client compatibility. For compatibility reports, include the operating system, client version and sanitized reproduction steps. Keep SSH credentials, private keys and active subscription URLs private.

Released under the [MIT License](LICENSE).
