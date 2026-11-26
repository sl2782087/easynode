package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

//go:embed switch-domain.sh
var switchDomainScript string

const domainSocket = "/run/easynodedeck-domain.sock"

func publicIP(ip netip.Addr) bool {
	return ip.IsValid() && ip.IsGlobalUnicast() && !ip.IsPrivate()
}

func hostIPs(host string) []netip.Addr {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	ips, _ := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	return ips
}

func panelIP(host string) string {
	var first string
	for _, addr := range hostIPs(host) {
		ip := addr.Unmap()
		if publicIP(ip) {
			if ip.Is4() {
				return ip.String()
			}
			first = ip.String()
		}
	}
	return first
}

func pointsHere(answers []netip.Addr, allowed map[netip.Addr]bool) bool {
	for _, addr := range answers {
		ip := addr.Unmap()
		if !publicIP(ip) || !allowed[ip] {
			return false
		}
	}
	return len(answers) > 0
}

// systemd supplies a private Unix socket on stdin/stdout and a root-owned EnvironmentFile.
func domainHelper() {
	reader := bufio.NewReader(io.LimitReader(os.Stdin, 256))
	line, err := reader.ReadString('\n')
	host, invalid := normalizeTarget(line)
	if err != nil || invalid != nil {
		fmt.Println("请输入完整域名 / Enter a valid hostname")
		return
	}
	current, err := url.Parse(os.Getenv("PUBLIC_URL"))
	if err != nil || current.Hostname() == "" {
		fmt.Println("面板地址配置异常 / Invalid panel configuration")
		return
	}
	allowed := make(map[netip.Addr]bool)
	ifaces, _ := net.InterfaceAddrs()
	for _, iface := range ifaces {
		if prefix, err := netip.ParsePrefix(iface.String()); err == nil {
			allowed[prefix.Addr().Unmap()] = true
		}
	}
	for _, literal := range []string{current.Hostname(), os.Getenv("PANEL_IPV4")} {
		if ip, err := netip.ParseAddr(literal); err == nil {
			allowed[ip.Unmap()] = true
		}
	}
	answers := hostIPs(host)
	if !pointsHere(answers, allowed) {
		fmt.Println("请将域名的所有 A / AAAA 记录直接指向本机，等待 DNS 生效后重试 / Point all A and AAAA records directly at this VPS")
		return
	}
	ipv4 := ""
	for _, ip := range answers {
		if ip.Unmap().Is4() {
			ipv4 = ip.Unmap().String()
			break
		}
	}
	cmd := exec.Command("/bin/bash", "-c", switchDomainScript, "switch-domain", host, ipv4)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func (a *App) bindDomain(w http.ResponseWriter, raw string) {
	host, err := normalizeTarget(raw)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	conn, err := net.DialTimeout("unix", domainSocket, 3*time.Second)
	if err != nil {
		http.Error(w, "域名服务未安装或不可用 / Domain service unavailable", 503)
		return
	}
	defer conn.Close()
	deadline := time.Now().Add(3 * time.Minute)
	_ = http.NewResponseController(w).SetWriteDeadline(deadline.Add(5 * time.Second))
	_ = conn.SetDeadline(deadline)
	if _, err := fmt.Fprintln(conn, host); err != nil {
		http.Error(w, "域名服务不可用 / Domain service unavailable", 503)
		return
	}
	reply, err := bufio.NewReader(io.LimitReader(conn, 2048)).ReadString('\n')
	if err != nil {
		http.Error(w, "域名服务超时或中断 / Domain service timed out or stopped", 503)
		return
	}
	if reply != "ready\n" && reply != "current\n" {
		http.Error(w, strings.TrimSpace(reply), 400)
		return
	}
	payload, _ := json.Marshal(object{"url": "https://" + host + "/web/", "current": reply == "current\n"})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	_, _ = w.Write(payload)
	_ = http.NewResponseController(w).Flush()
	// Closing the socket releases the helper to restart after this response.
}
