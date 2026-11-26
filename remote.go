package main

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

//go:embed provision-node.sh
var nodeInstaller string

//go:embed install.sh
var panelInstaller string

type Remote struct {
	ID, Host, Fingerprint, SNI string
	Port                       int
	Ports                      [3]int
	Node                       Node
	Checked                    time.Time
}

type portCheck struct {
	at time.Time
	ok bool
}

// Probe the configured TCP listeners outside the state lock. For UDP-only nodes
// retain the timestamp of the last service check over SSH.
func (a *App) monitorRemotes() {
	for {
		a.Lock()
		remotes := slices.Clone(a.state.Remotes)
		a.Unlock()
		for _, r := range remotes {
			result := portCheck{ok: true}
			for _, p := range r.Node.Lines {
				if p == 2 {
					continue
				}
				result.at = time.Now()
				conn, err := net.DialTimeout("tcp", net.JoinHostPort(r.Host, strconv.Itoa(r.Ports[p])), 3*time.Second)
				if err != nil {
					result.ok = false
					break
				}
				conn.Close()
			}
			if !result.at.IsZero() {
				a.Lock()
				if a.reachability == nil {
					a.reachability = make(map[string]portCheck)
				}
				a.reachability[r.ID] = result
				a.Unlock()
			}
		}
		time.Sleep(time.Minute)
	}
}

// A first connection stops at host-key verification, before SSH authentication.
func connectRemote(in input, probe bool) (*ssh.Client, string, error) {
	target := net.JoinHostPort(in.Host, strconv.Itoa(in.Port))
	conn, err := net.DialTimeout("tcp", target, 8*time.Second)
	if err != nil {
		return nil, "", fmt.Errorf("SSH connection to %s failed", target)
	}
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	var fingerprint string
	stop := errors.New("host key obtained")
	cfg := ssh.ClientConfig{User: "root", HostKeyAlgorithms: []string{ssh.KeyAlgoED25519, ssh.KeyAlgoECDSA256, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}, HostKeyCallback: func(_ string, _ net.Addr, k ssh.PublicKey) error {
		fingerprint = ssh.FingerprintSHA256(k)
		if probe {
			return stop
		}
		if fingerprint != in.Fingerprint {
			return errors.New("SSH host key mismatch")
		}
		return nil
	}}
	if !probe {
		cfg.Auth = []ssh.AuthMethod{ssh.Password(in.Password)}
	}
	c, ch, requests, err := ssh.NewClientConn(conn, target, &cfg)
	if err != nil {
		conn.Close()
		if probe && errors.Is(err, stop) {
			return nil, fingerprint, nil
		}
		if !probe && fingerprint != "" && fingerprint != in.Fingerprint {
			return nil, "", errors.New("SSH host key changed; verify the host")
		}
		return nil, "", errors.New("SSH handshake or password authentication failed")
	}
	_ = conn.SetDeadline(time.Now().Add(8 * time.Minute))
	return ssh.NewClient(c, ch, requests), fingerprint, nil
}

// Run a fixed operation. Sensitive parameters travel in the SSH stdin stream,
// never in the server's process arguments.
func remoteScript(c *ssh.Client, script string) ([]byte, error) {
	s, err := c.NewSession()
	if err != nil {
		return nil, err
	}
	defer s.Close()
	s.Stdin = strings.NewReader(script)
	output, err := s.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = s.Start("bash -se"); err != nil {
		return nil, err
	}
	const limit = 8192
	raw, err := io.ReadAll(io.LimitReader(output, limit+1))
	if err != nil || len(raw) > limit {
		return nil, errors.New("Invalid remote response")
	}
	if err = s.Wait(); err != nil {
		var exit *ssh.ExitError
		if errors.As(err, &exit) && exit.ExitStatus() == 78 {
			return nil, errors.New("远程系统不符合安装要求，支持版本见 README / See README for supported versions, CPU architectures and systemd requirements")
		}
		return nil, fmt.Errorf("remote setup failed (%v); inspect /var/log/easynodedeck-node-install.log on the node", err)
	}
	return raw, nil
}

const identityScript = `set -eu
umask 077
test "$(id -u)" = 0
install -d -m 0700 /etc/easynodedeck-node
cd /etc/easynodedeck-node
exec 9>.lock
flock -w 30 9
if ! test -s identity.json; then
  echo 'SEED_BASE64' | base64 -d > identity.new
  mv identity.new identity.json
fi
cat identity.json
`

func (a *App) manageRemote(path string, in input) (object, error) {
	ip, err := netip.ParseAddr(in.Host)
	if err != nil || !ip.Is4() || !ip.IsGlobalUnicast() || ip.IsPrivate() || in.Port < 1 || in.Port > 65535 || in.Host == a.host {
		return nil, errors.New("Enter a remote public IPv4 address and SSH port")
	}
	if path == "remote/key" {
		_, fp, err := connectRemote(in, true)
		return object{"fingerprint": fp}, err
	}
	if path != "remote/save" && path != "remote/delete" {
		return nil, errors.New("Unknown remote operation")
	}
	if in.Password == "" || in.Fingerprint == "" {
		return nil, errors.New("SSH password and verified fingerprint are required")
	}
	if !a.remoteDeploy.TryLock() {
		return nil, errors.New("Another deployment is in progress")
	}
	defer a.remoteDeploy.Unlock()
	a.Lock()
	index := slices.IndexFunc(a.state.Remotes, func(r Remote) bool { return r.Host == in.Host })
	duplicateName := slices.ContainsFunc(a.state.Remotes, func(r Remote) bool { return r.Host != in.Host && r.Node.Name == strings.TrimSpace(in.Name) })
	var remote Remote
	if index >= 0 {
		remote = a.state.Remotes[index]
	}
	a.Unlock()
	if in.ID != "" && remote.ID != in.ID {
		return nil, errors.New("Remote node identity mismatch")
	}
	if remote.ID != "" && remote.Fingerprint != in.Fingerprint {
		return nil, errors.New("Saved SSH host key does not match")
	}
	var lines []int
	if path == "remote/save" {
		if duplicateName {
			return nil, errors.New("Choose a unique remote node name")
		}
		if !validName(in.Name) || len(in.Lines) == 0 {
			return nil, errors.New("Choose a name and protocols")
		}
		for _, key := range in.Lines {
			p, err := strconv.Atoi(key)
			if err != nil || p < 0 || p >= len(protocols) || slices.Contains(lines, p) {
				return nil, errors.New("Invalid protocol")
			}
			lines = append(lines, p)
		}
		slices.Sort(lines)
	} else if remote.ID == "" {
		return nil, errors.New("Unknown remote node")
	}
	client, fp, err := connectRemote(in, false)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	if path == "remote/delete" {
		_, err = remoteScript(client, `set -eu
test -s /etc/easynodedeck-node/identity.json
systemctl disable --now easynodedeck-node.service
if test -f /etc/systemd/system/easynodedeck-node-renew.timer; then systemctl disable --now easynodedeck-node-renew.timer; fi`)
		if err != nil {
			return nil, err
		}
	} else {
		mode := "--prepare-node"
		if slices.Contains(lines, 2) {
			mode = "--prepare-node-tls"
		}
		// Redirect package-manager output before it reaches the bounded SSH response.
		if _, err = remoteScript(client, "umask 077\nexec >>/var/log/easynodedeck-node-install.log 2>&1\nset -- "+mode+"\n"+panelInstaller); err != nil {
			return nil, err
		}
		seed, err := newNode()
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(seed)
		raw, err := remoteScript(client, strings.Replace(identityScript, "SEED_BASE64", base64.StdEncoding.EncodeToString(b), 1))
		if err != nil {
			return nil, err
		}
		var n Node
		if json.Unmarshal(raw, &n) != nil || n.UUID == "" || n.SS == "" || n.Private == "" || n.Public == "" || n.HY == "" {
			return nil, errors.New("Invalid saved node identity")
		}
		n.Name = strings.TrimSpace(in.Name)
		n.Lines = lines
		if remote.ID == "" {
			remote.ID = random(12)
			remote.Ports = [3]int{30443, 30444, 30445}
			remote.SNI = a.sni
		}
		remote.Host, remote.Port, remote.Fingerprint, remote.Node = in.Host, in.Port, fp, n
		endpoint := App{ports: remote.Ports, sni: remote.SNI, cert: "/run/credentials/easynodedeck-node.service/cert.pem", key: "/run/credentials/easynodedeck-node.service/key.pem"}
		inbounds := []object{}
		for _, p := range lines {
			inbounds = append(inbounds, endpoint.inbound(&n, p))
		}
		config, _ := json.Marshal(object{"log": object{"level": "error"}, "inbounds": inbounds, "outbounds": []object{{"type": "direct", "tag": "direct"}}})
		// Host is validated as IPv4. The other two parameters contain only base64 and a boolean.
		arguments := fmt.Sprintf("NODE_HOST=%s\nWITH_TLS=%t\nNODE_CONFIG='%s'\n", in.Host, slices.Contains(lines, 2), base64.StdEncoding.EncodeToString(config))
		if _, err = remoteScript(client, arguments+nodeInstaller); err != nil {
			return nil, err
		}
		remote.Checked = time.Now()
		remote.Node.Private = "" // Only the remote service needs its REALITY private key.
	}
	a.Lock()
	defer a.Unlock()
	state := a.state
	state.Remotes = slices.Clone(state.Remotes)
	if path == "remote/delete" {
		state.Remotes = slices.Delete(state.Remotes, index, index+1)
	} else if index >= 0 {
		if prev := state.Remotes[index].Node; prev.UUID == remote.Node.UUID && slices.Equal(prev.Lines, remote.Node.Lines) {
			remote.Node.Network = prev.Network
		}
		state.Remotes[index] = remote
	} else {
		state.Remotes = append(state.Remotes, remote)
	}
	state.Subs = slices.Clone(state.Subs)
	for i, sub := range state.Subs {
		state.Subs[i].Lines = slices.DeleteFunc(slices.Clone(sub.Lines), func(l string) bool { return !state.hasLine(l) })
	}
	return nil, a.save(state)
}
