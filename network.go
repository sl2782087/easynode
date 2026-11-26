package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

type ExitAddress struct {
	IP           string `json:"ip"`
	Location     string `json:"location"`
	ASN          string `json:"asn"`
	Organization string `json:"organization"`
	PTR          string `json:"ptr,omitempty"`
}

type NetworkInfo struct {
	Addresses []ExitAddress `json:"addresses"`
	Checked   time.Time     `json:"checked"`
	Protocol  int           `json:"protocol"`
	TCPMillis *int64        `json:"tcpMillis,omitempty"`
}

type networkTarget struct {
	node      Node
	host, sni string
	ports     [3]int
}

func (a *App) networkTarget(id string) (networkTarget, bool) {
	if id == "local" && a.state.Node != nil {
		return networkTarget{*a.state.Node, a.host, a.sni, a.ports}, true
	}
	for _, remote := range a.state.Remotes {
		if remote.ID == id {
			return networkTarget{remote.Node, remote.Host, remote.SNI, remote.Ports}, true
		}
	}
	return networkTarget{}, false
}

func (t networkTarget) outbound(p int) object {
	out := object{"tag": "node", "server": t.host, "server_port": t.ports[p]}
	switch p {
	case 0:
		out["type"], out["method"], out["password"] = "shadowsocks", cipherName, t.node.SS
	case 1:
		out["type"], out["uuid"], out["flow"] = "vless", t.node.UUID, "xtls-rprx-vision"
		out["tls"] = object{"enabled": true, "server_name": t.sni, "utls": object{"enabled": true, "fingerprint": "chrome"}, "reality": object{"enabled": true, "public_key": t.node.Public, "short_id": t.node.Short}}
	case 2:
		out["type"], out["password"] = "hysteria2", t.node.HY
		out["tls"] = object{"enabled": true, "server_name": t.host}
	}
	return out
}

func parseGeo(raw []byte, ipv4 bool) (ExitAddress, bool) {
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 4 {
		return ExitAddress{}, false
	}
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	ip, err := netip.ParseAddr(lines[0])
	if err != nil || ip.Is4() != ipv4 || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return ExitAddress{}, false
	}
	return ExitAddress{IP: ip.String(), Location: lines[1], ASN: lines[2], Organization: lines[3]}, true
}

func fetchExit(ctx context.Context, binary string, config []byte, ipv4 bool) (ExitAddress, bool) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	host := "ipv6.ping0.cc"
	if ipv4 {
		host = "ipv4.ping0.cc"
	}
	cmd := exec.CommandContext(ctx, binary, "-c", "/dev/stdin", "tools", "fetch", "-o", "node", "https://"+host+"/geo")
	cmd.Stdin = bytes.NewReader(config)
	stdout, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return ExitAddress{}, false
	}
	raw, err := io.ReadAll(io.LimitReader(stdout, 4097))
	if err != nil || len(raw) > 4096 {
		cancel()
	}
	if cmd.Wait() != nil || err != nil || len(raw) > 4096 {
		return ExitAddress{}, false
	}
	return parseGeo(raw, ipv4)
}

func (a *App) refreshNetwork(ctx context.Context, id string) error {
	a.Lock()
	target, ok := a.networkTarget(id)
	if !ok || len(target.node.Lines) == 0 {
		a.Unlock()
		return errors.New("节点不存在 / Node unavailable")
	}
	if a.checking[id] {
		a.Unlock()
		return errors.New("检测中 / Check already running")
	}
	if a.checking == nil {
		a.checking = map[string]bool{}
	}
	a.checking[id] = true
	a.Unlock()
	defer func() { a.Lock(); delete(a.checking, id); a.Unlock() }()
	p := target.node.Lines[0]
	config, _ := json.Marshal(object{"log": object{"disabled": true}, "outbounds": []object{target.outbound(p)}})
	info := &NetworkInfo{Protocol: p}
	if p != 2 {
		start := time.Now()
		dialer := net.Dialer{Timeout: 2 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(target.host, strconv.Itoa(target.ports[p])))
		if err == nil {
			ms := time.Since(start).Milliseconds()
			info.TCPMillis = &ms
			conn.Close()
		}
	}
	for _, ipv4 := range []bool{true, false} {
		addr, ok := fetchExit(ctx, a.binary, config, ipv4)
		if !ok {
			continue
		}
		lookup, cancel := context.WithTimeout(ctx, time.Second)
		names, err := net.DefaultResolver.LookupAddr(lookup, addr.IP)
		cancel()
		if err == nil && len(names) > 0 {
			addr.PTR = strings.TrimSuffix(names[0], ".")
		}
		info.Addresses = append(info.Addresses, addr)
	}
	if len(info.Addresses) == 0 {
		return errors.New("未能通过该线路获取 Ping0 数据 / No Ping0 response through this connection")
	}
	info.Checked = time.Now().UTC()
	a.Lock()
	defer a.Unlock()
	current, ok := a.networkTarget(id)
	if !ok || current.node.UUID != target.node.UUID || !slices.Equal(current.node.Lines, target.node.Lines) {
		return errors.New("节点已变更 / Node changed")
	}
	state := a.state
	if id == "local" {
		node := *state.Node
		node.Network = info
		state.Node = &node
	} else {
		state.Remotes = slices.Clone(state.Remotes)
		for i := range state.Remotes {
			if state.Remotes[i].ID == id {
				state.Remotes[i].Node.Network = info
				break
			}
		}
	}
	return a.save(state)
}

// Older installations have no cached observations. Backfill them once on startup.
func (a *App) fillMissingNetworks() {
	a.Lock()
	var ids []string
	if n := a.state.Node; n != nil && n.Network == nil {
		ids = append(ids, "local")
	}
	for _, r := range a.state.Remotes {
		if r.Node.Network == nil {
			ids = append(ids, r.ID)
		}
	}
	a.Unlock()
	for _, id := range ids {
		_ = a.refreshNetwork(context.Background(), id)
	}
}
