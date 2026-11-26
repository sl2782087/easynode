<p align="center"><img src="assets/logo.png" width="96" height="96" alt="EasyNodeDeck"></p>

# EasyNodeDeck · 易节点

简体中文 · [English](README.en.md)

在自己的 Linux 服务器上运行节点，勾选线路即可生成订阅。当前本机节点版支持 Shadowsocks、VLESS + REALITY 和 Hysteria 2，提供固定订阅地址及二维码。

## 安装

在 Debian 12（amd64 或 arm64）上以 root 身份执行。安装时可填写公网 IPv4，或填写已经解析到服务器的域名。用户名和密码留空会随机生成。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/sl2782087/easynodedeck/v0.1.0/install.sh)
```

TCP 80 用于证书验证和续期，TCP 443 用于管理面板和订阅。默认线路端口为 SS 30443 TCP/UDP、REALITY 30444 TCP、HY2 30445 UDP。云防火墙需要放行所选协议对应的端口。安装器会申请受信任证书并配置 systemd 自动续期。

## 生成可用订阅

1. 打开安装结束时显示的 HTTPS 地址并登录。初始账密也会保存在仅 root 可读的 `/etc/easynodedeck/initial-access`。
2. 在「节点」点击「添加本机节点」，填写名称并勾选协议。保存后启动独立的 sing-box 实例。重复保存沿用原有凭据，机器上的其他服务可继续运行。
3. 在「订阅」点击「创建订阅」，选择线路，复制地址或扫描二维码，在 Clash Verge Rev 或 Shadowrocket 中导入。调整线路后刷新原地址即可。需要指定输出格式时，在地址末尾加 `?format=clash` 或 `?format=uri`。
4. 在「用户中心」可以修改管理员用户名和密码。删除订阅会撤销下载地址，已下载的节点凭据会继续有效，直至底层节点凭据更换。

面板以独立系统用户运行。Go 程序内嵌网页并管理本机节点进程。单管理员数据以私有 JSON 文件原子保存至 `/var/lib/easynodedeck/state.json`，备份此文件可以保留节点凭据和订阅地址。用 `systemctl status easynodedeck` 检查运行状态。在 `/etc/easynodedeck/panel.env` 中可指定 `SS_PORT`、`REALITY_PORT`、`HY2_PORT` 和 `REALITY_SNI`，修改后重启服务。

远程 SSH 部署和第三方线路导入留待后续版本。浏览器直接打开本地 [web/index.html](web/index.html) 可试用离线设计预览。

![节点管理面板预览](assets/panel-preview.webp)

## 参与

欢迎用中文或英文在 [Issues](https://github.com/sl2782087/easynodedeck/issues) 讨论安装体验、节点管理和客户端兼容性。反馈兼容问题时，请附操作系统、客户端版本和脱敏后的复现步骤。请勿公开 SSH 凭据、私钥或真实订阅地址。

本项目采用 [MIT 许可证](LICENSE)。
