<p align="center"><img src="assets/logo.png" width="96" height="96" alt="EasyNodeDeck"></p>

# EasyNodeDeck · 易节点

简体中文 · [English](README.en.md)

为个人 VPS 设计的轻量自托管代理面板。在一个页面管理本机和远程服务器，把选中的线路分享给 Clash Verge Rev 或 Shadowrocket。

**开发状态**　项目筹备中。首个可安装版本的范围如下。

## 第一版准备做什么

1. **安装面板**　在 Linux 服务器执行一条命令。可指定管理员账密，留空则随机生成。完成后显示登录地址和初始账密。默认使用公网 IP，域名可在安装时填写或稍后绑定。
2. **添加节点**　选择本机或填写远程 SSH 连接信息，再选择 Shadowsocks、VLESS + REALITY 或 Hysteria 2。一台服务器可承载多条线路。重新添加由面板部署的节点时，保留现有凭据和配置。
3. **生成订阅**　勾选线路，获取固定的订阅地址及二维码。调整线路后，客户端刷新原订阅即可同步。已有第三方节点可通过分享链接加入订阅。

## 实现方向

管理面板计划使用 Go、内嵌网页和 SQLite。SSH 负责远程部署，由 systemd 管理节点上的 sing-box。面板机可单独承担管理任务，也可启用本机节点。首版将以 1 核、512 MB 内存的 Linux VPS 作为纯面板验收环境，实际占用随首版公布。

## 参与

欢迎用中文或英文在 [Issues](https://github.com/sl2782087/easynodedeck/issues) 讨论安装体验、节点管理和客户端兼容性。反馈兼容问题时，请附操作系统、客户端版本和脱敏后的复现步骤。请勿公开 SSH 凭据、私钥或真实订阅地址。

本项目采用 [MIT 许可证](LICENSE)。
