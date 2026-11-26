package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubscriptionLifecycle(t *testing.T) {
	a := &App{dir: t.TempDir(), host: "example.org", ports: [3]int{31000, 31001, 31002}}
	node, err := newNode()
	if err != nil {
		t.Fatal(err)
	}
	node.Name, node.Lines = "Local", []int{0, 1, 2}
	a.state.Node = node
	in := input{Name: "Selected", Lines: []string{"local/0", "local/2"}}
	if err := a.change("sub", in); err != nil {
		t.Fatal(err)
	}
	sub := a.state.Subs[0]
	raw, err := base64.StdEncoding.DecodeString(a.subscription(sub, false))
	if err != nil || !strings.HasPrefix(string(raw), "ss://") || !strings.Contains(string(raw), "\nhysteria2://") || strings.Contains(string(raw), "vless://") {
		t.Fatal("Unexpected URI selection")
	}
	info, err := os.Stat(filepath.Join(a.dir, "state.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("State must be private")
	}
	in.ID, in.Lines = sub.ID, []string{"local/1"}
	if err := a.change("sub", in); err != nil || a.state.Subs[0].ID != sub.ID {
		t.Fatal("Editing must preserve the subscription URL")
	}
	if err := a.change("node/delete", input{}); err != nil || len(a.state.Subs[0].Lines) != 0 {
		t.Fatal("Deleted node must be removed from subscriptions")
	}
	if strings.Contains(a.subscription(a.state.Subs[0], true), "MATCH,DIRECT") {
		t.Fatal("An empty profile must not send traffic directly")
	}
	if err := a.change("sub/delete", in); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	a.serveSubscription(response, httptest.NewRequest("GET", "/s/"+sub.ID, nil))
	if response.Code != http.StatusNotFound {
		t.Fatal("Deleted subscription must be unavailable")
	}
}

func TestManagementRequiresSession(t *testing.T) {
	a := &App{origin: "https://example.org"}
	for _, tc := range []struct {
		method, path, origin string
		status               int
	}{
		{"GET", "/api/state", "", 401},
		{"POST", "/api/node", "https://example.org", 401},
		{"POST", "/api/node", "https://other.example", 403},
		{"POST", "/api/remote/key", "https://example.org", 401},
		{"POST", "/api/remote/save", "https://other.example", 403},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		a.api(res, req)
		if res.Code != tc.status {
			t.Fatalf("%s: got %d, want %d", tc.path, res.Code, tc.status)
		}
	}
}

func TestMixedSubscriptionSurvivesLocalRemoval(t *testing.T) {
	a := App{dir: t.TempDir(), host: "local.example", ports: [3]int{30443, 30444, 30445}}
	n, err := newNode()
	if err != nil {
		t.Fatal(err)
	}
	n.Name = "Panel"
	n.Lines = []int{0}
	a.state.Node = n
	r := Remote{ID: "remote1", Host: "remote.example", Ports: [3]int{30443, 30444, 30445}, Node: *n}
	r.Node.Name = "Remote"
	a.state.Remotes = []Remote{r}
	if err := a.change("sub", input{Name: "Both", Lines: []string{"local/0", "remote1/0"}}); err != nil {
		t.Fatal(err)
	}
	s := a.state.Subs[0]
	profile := a.subscription(s, true)
	if !strings.Contains(profile, "Panel · 本机 · Shadowsocks") || !strings.Contains(profile, "Remote · Shadowsocks") || strings.Contains(profile, "Remote · 本机") {
		t.Fatal("wrong origin labels")
	}
	if err := a.change("node/delete", input{}); err != nil {
		t.Fatal(err)
	}
	if a.state.Subs[0].ID != s.ID || len(a.state.Subs[0].Lines) != 1 || a.state.Subs[0].Lines[0] != "remote1/0" {
		t.Fatal("remote selection lost")
	}
}
