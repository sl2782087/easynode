package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestRealitySettings(t *testing.T) {
	for _, tc := range []struct {
		s          RealitySettings
		management int
		ok         bool
	}{
		{RealitySettings{"Dl.Google.com.", 30444}, 443, true},
		{RealitySettings{"example.org", 443}, 22, true},
		{RealitySettings{"example.org", 443}, 443, false},
		{RealitySettings{"example.org", 30443}, 22, false},
		{RealitySettings{"example.org", 2222}, 2222, false},
		{RealitySettings{"https://example.org/path", 443}, 22, false},
		{RealitySettings{"127.0.0.1", 30444}, 22, false},
	} {
		s, err := tc.s.validate([]int{0, 1}, 30443, tc.management)
		if (err == nil) != tc.ok {
			t.Fatal(tc, err)
		}
		if err == nil && strings.HasSuffix(s.Target, ".") {
			t.Fatal("unnormalized")
		}
	}
}
func TestRealityOverridesReachAllConsumers(t *testing.T) {
	n, _ := newNode()
	n.Name = "demo"
	n.Lines = []int{1}
	a := App{host: "demo.example", ports: [3]int{30443, 30444, 30445}, sni: "www.cloudflare.com", state: State{Node: n}}
	if e := a.localEndpoint(n); e.ports[1] != 30444 || e.sni != a.sni {
		t.Fatal("legacy defaults")
	}
	n.Reality = RealitySettings{"dl.google.com", 32444}
	e := a.localEndpoint(n)
	in := e.inbound(n, 1)
	if in["listen_port"] != 32444 || in["tls"].(object)["server_name"] != "dl.google.com" {
		t.Fatal("server mismatch")
	}
	sub := Subscription{Lines: []string{"local/1"}}
	uri, err := base64.StdEncoding.DecodeString(a.subscription(sub, false))
	if err != nil || !strings.Contains(string(uri), ":32444?") || !strings.Contains(string(uri), "sni=dl.google.com") {
		t.Fatal("URI mismatch")
	}
	profile := a.subscription(sub, true)
	if !strings.Contains(profile, `"servername":"dl.google.com"`) || !strings.Contains(profile, `"port":32444`) {
		t.Fatal("Clash mismatch")
	}
	target, _ := a.networkTarget("local")
	if target.sni != e.sni || target.ports != e.ports {
		t.Fatal("probe mismatch")
	}
}
