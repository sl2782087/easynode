package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionLifecycleAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	a := &App{dir: dir, state: State{User: "admin", Salt: "test", Hash: digest("original", "test")}}
	call := func(path, body string, cookie *http.Cookie, status int) *http.Response {
		t.Helper()
		method := "POST"
		if body == "" {
			method = "GET"
		}
		r := httptest.NewRequest(method, "/api/"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		a.api(w, r)
		if w.Code != status {
			t.Fatalf("%s returned %d, want %d", path, w.Code, status)
		}
		return w.Result()
	}
	restart := func() {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
		if err != nil {
			t.Fatal(err)
		}
		a = &App{dir: dir}
		if err := json.Unmarshal(raw, &a.state); err != nil {
			t.Fatal(err)
		}
	}
	login := func() *http.Cookie {
		return call("login", `{"User":"admin","Password":"original"}`, nil, 200).Cookies()[0]
	}
	first, second := login(), login()
	if _, ok := a.state.Sessions[first.Value]; ok {
		t.Fatal("raw session token persisted")
	}
	restart()
	call("state", "", first, 200)
	call("state", "", second, 200)
	expired := &http.Cookie{Name: first.Name, Value: "expired"}
	a.state.Sessions[sessionKey(expired.Value)] = time.Now().Add(-time.Minute)
	call("state", "", expired, 401)
	logout := call("logout", "{}", first, 204)
	if logout.Cookies()[0].MaxAge >= 0 {
		t.Fatal("missing browser cookie removal")
	}
	restart()
	call("state", "", first, 401)
	other := login()
	next := call("account", `{"Current":"original","Password":"replacement"}`, second, 200).Cookies()[0]
	restart()
	call("state", "", second, 401)
	call("state", "", other, 401)
	call("state", "", next, 200)
}
