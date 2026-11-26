<p align="center"><img src="assets/logo.png" width="96" height="96" alt="EasyNodeDeck"></p>

# EasyNodeDeck

[简体中文](README.md) · English

A lightweight, self-hosted proxy panel for personal VPS fleets. Manage local and remote servers in one place and share selected connections with Clash Verge Rev or Shadowrocket.

**Status**　Early development. The first installable release is planned around the workflow below.

## First release

1. **Install the panel.** Run one command on a Linux server. Choose administrator credentials or leave the fields blank to generate them. The installer prints the sign-in URL and initial credentials. Use a public IP by default, or bind a domain during setup or later.
2. **Add a node.** Select the panel host or enter SSH connection details for a remote server. Choose Shadowsocks, VLESS + REALITY or Hysteria 2. Each server can host several connections. Adding a previously managed node again should preserve its credentials and configuration.
3. **Create a subscription.** Select connections to get a stable subscription URL and QR code. Clients can refresh the original URL after the selection changes. Existing third-party connections can join a subscription through their share links.

## Implementation plan

The panel will use Go, embedded web assets and SQLite. It will deploy over SSH and run sing-box under systemd on each node. The panel host can also serve as a node. The first release will be tested on a Linux VPS with one CPU core and 512 MB of RAM running the panel alone. Measurements will accompany the release.

## Contributing

We welcome [issues](https://github.com/sl2782087/easynodedeck/issues) in English or Chinese about installation, node management and client compatibility. For compatibility reports, include the operating system, client version and sanitized reproduction steps. Keep SSH credentials, private keys and active subscription URLs private.

Released under the [MIT License](LICENSE).
