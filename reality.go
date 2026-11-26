package main

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Unset local overrides follow the panel's environment defaults (also for older state files).
type RealitySettings struct {
	Target string `json:"target"`
	Port   int    `json:"port"`
}

func (a *App) localEndpoint(n *Node) *App {
	e := &App{host: a.host, ports: a.ports, sni: a.sni, cert: a.cert, key: a.key}
	if n != nil {
		if n.Reality.Target != "" {
			e.sni = n.Reality.Target
		}
		if n.Reality.Port != 0 {
			e.ports[1] = n.Reality.Port
		}
	}
	return e
}

var dnsName = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$`)

func normalizeTarget(host string) (string, error) {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if len(host) > 253 || !dnsName.MatchString(host) || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return "", errors.New("请输入完整公网域名 / Enter a public hostname without a URL or port")
	}
	return host, nil
}

func (s RealitySettings) validate(lines []int, ssPort, managementPort int) (RealitySettings, error) {
	host, err := normalizeTarget(s.Target)
	if err != nil {
		return s, err
	}
	if (s.Port < 1024 && s.Port != 443) || s.Port > 65535 || s.Port == managementPort || (s.Port == ssPort && slices.Contains(lines, 0)) {
		return s, errors.New("端口不可用，请选择未占用的 1024–65535 或独立节点的 443 / Invalid or conflicting TCP port")
	}
	return RealitySettings{host, s.Port}, nil
}

// The optional pre-save check verifies the certificate, TLS 1.3 and negotiated HTTP/2.
type targetDial func(context.Context, string, string) (net.Conn, error)

func checkTarget(ctx context.Context, host string, dial targetDial) error {
	host, err := normalizeTarget(host)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	c, err := dial(ctx, "tcp", net.JoinHostPort(host, "443"))
	if err != nil {
		return errors.New("节点无法连接目标 443 / Node could not reach target port 443")
	}
	defer c.Close()
	conn := tls.Client(c, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS13, NextProtos: []string{"h2"}})
	if err := conn.HandshakeContext(ctx); err != nil {
		return errors.New("TLS 1.3 握手或证书验证失败 / TLS 1.3 handshake or certificate verification failed")
	}
	if conn.ConnectionState().NegotiatedProtocol != "h2" {
		return errors.New("目标未协商 HTTP/2 / Target did not negotiate HTTP/2")
	}
	return nil
}
