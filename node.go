package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type object = map[string]any

var protocols = []string{"Shadowsocks", "VLESS + REALITY", "Hysteria 2"}

const cipherName = "chacha20-ietf-poly1305"

type Node struct {
	Name                                 string
	Lines                                []int
	SS, UUID, Private, Public, Short, HY string
}
type Subscription struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Lines []string `json:"lines"`
}
type State struct {
	User, Salt, Hash string
	Node             *Node
	Subs             []Subscription
	Remotes          []Remote
}

func random(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func newNode() (*Node, error) {
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	uuid := random(16)
	return &Node{SS: random(24), HY: random(24), UUID: uuid[:8] + "-" + uuid[8:12] + "-4" + uuid[13:16] + "-a" + uuid[17:20] + "-" + uuid[20:], Short: random(8), Private: base64.RawURLEncoding.EncodeToString(k.Bytes()), Public: base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes())}, nil
}
func (a *App) inbound(n *Node, p int) object {
	in := object{"listen": "::", "listen_port": a.ports[p], "tag": fmt.Sprintf("local-%d", p)}
	switch p {
	case 0:
		in["type"] = "shadowsocks"
		in["method"] = cipherName
		in["password"] = n.SS
	case 1:
		in["type"] = "vless"
		in["users"] = []object{{"uuid": n.UUID, "flow": "xtls-rprx-vision"}}
		in["tls"] = object{"enabled": true, "server_name": a.sni, "reality": object{"enabled": true, "private_key": n.Private, "short_id": []string{n.Short}, "handshake": object{"server": a.sni, "server_port": 443}}}
	case 2:
		in["type"] = "hysteria2"
		in["users"] = []object{{"password": n.HY}}
		in["tls"] = object{"enabled": true, "certificate_path": a.cert, "key_path": a.key}
	}
	return in
}
func (a *App) link(n *Node, p int, local bool) (object, string) {
	port := a.ports[p]
	addr := net.JoinHostPort(a.host, strconv.Itoa(port))
	name := n.Name
	if local {
		name += " · 本机"
	}
	name += " · " + protocols[p]
	label := "#" + url.PathEscape(name)
	m := object{"name": name, "server": a.host, "port": port, "udp": true}
	switch p {
	case 0:
		m["type"] = "ss"
		m["cipher"] = cipherName
		m["password"] = n.SS
		return m, "ss://" + base64.RawURLEncoding.EncodeToString([]byte(cipherName+":"+n.SS)) + "@" + addr + label
	case 1:
		m["type"] = "vless"
		m["uuid"] = n.UUID
		m["flow"] = "xtls-rprx-vision"
		m["tls"] = true
		m["servername"] = a.sni
		m["client-fingerprint"] = "chrome"
		m["reality-opts"] = object{"public-key": n.Public, "short-id": n.Short}
		q := url.Values{"security": {"reality"}, "type": {"tcp"}, "encryption": {"none"}, "sni": {a.sni}, "pbk": {n.Public}, "sid": {n.Short}, "fp": {"chrome"}, "flow": {"xtls-rprx-vision"}}
		return m, "vless://" + n.UUID + "@" + addr + "?" + q.Encode() + label
	default:
		m["type"] = "hysteria2"
		m["password"] = n.HY
		return m, "hysteria2://" + n.HY + "@" + addr + label
	}
}
func (a *App) subscription(s Subscription, clash bool) string {
	var proxies, uris, names []string
	appendNode := func(endpoint *App, id string, n *Node) {
		if n == nil {
			return
		}
		for _, p := range n.Lines {
			if slices.Contains(s.Lines, fmt.Sprintf("%s/%d", id, p)) {
				m, uri := endpoint.link(n, p, id == "local")
				data, _ := json.Marshal(m)
				proxies = append(proxies, "  - "+string(data))
				uris = append(uris, uri)
				names = append(names, m["name"].(string))
			}
		}
	}
	appendNode(a, "local", a.state.Node)
	for _, remote := range a.state.Remotes {
		endpoint := App{host: remote.Host, ports: remote.Ports, sni: remote.SNI}
		appendNode(&endpoint, remote.ID, &remote.Node)
	}
	if !clash {
		return base64.StdEncoding.EncodeToString([]byte(strings.Join(uris, "\n")))
	}
	if len(names) == 0 {
		return "proxies: []\nproxy-groups: []\nrules:\n  - MATCH,REJECT\n"
	}
	group, _ := json.Marshal(object{"name": "EasyNodeDeck", "type": "select", "proxies": names})
	return "mode: rule\nproxies:\n" + strings.Join(proxies, "\n") + "\nproxy-groups:\n  - " + string(group) + "\nrules:\n  - MATCH,EasyNodeDeck\n"
}
func atomicWrite(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".pending-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err != nil {
		return err
	}
	if cerr != nil {
		return cerr
	}
	return os.Rename(f.Name(), path)
}

type core struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func (c *core) alive() bool {
	if c == nil {
		return false
	}
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}
func (a *App) stop() {
	c := a.core
	if c.alive() {
		_ = c.cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-c.done:
		case <-time.After(3 * time.Second):
			_ = c.cmd.Process.Kill()
			<-c.done
		}
	}
	a.core = nil
}
func (a *App) apply(n *Node) error {
	path := filepath.Join(a.dir, "core.json")
	if n == nil {
		a.stop()
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	var ins []object
	for _, p := range n.Lines {
		ins = append(ins, a.inbound(n, p))
	}
	data, err := json.Marshal(object{"log": object{"level": "error"}, "inbounds": ins, "outbounds": []object{{"type": "direct", "tag": "direct"}}})
	if err != nil {
		return err
	}
	candidate := filepath.Join(a.dir, "candidate.json")
	defer os.Remove(candidate)
	if err = atomicWrite(candidate, data); err != nil {
		return err
	}
	if err = exec.Command(a.binary, "check", "-c", candidate).Run(); err != nil {
		return fmt.Errorf("core config check failed: %w", err)
	}
	if err = os.Rename(candidate, path); err != nil {
		return err
	}
	a.stop()
	cmd := exec.Command(a.binary, "run", "-c", path)
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		return err
	}
	c := &core{cmd, make(chan struct{})}
	a.core = c
	go func() { _ = cmd.Wait(); close(c.done) }()
	select {
	case <-c.done:
		return fmt.Errorf("core stopped during startup; check ports and journal")
	case <-time.After(450 * time.Millisecond):
		return nil
	}
}
