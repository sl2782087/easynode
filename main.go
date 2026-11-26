package main

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed web/* assets/logo.png
var files embed.FS

type App struct {
	sync.Mutex
	dir, host, origin, binary, sni, cert, key string
	ports                                     [3]int
	state                                     State
	core                                      *core
	failures                                  map[string]int
	reset                                     time.Time
	remoteDeploy                              sync.Mutex
	reachability                              map[string]portCheck
	checking                                  map[string]bool
}

func env(key, fallback string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return fallback
}
func digest(password, salt string) string {
	b, err := pbkdf2.Key(sha256.New, password, []byte(salt), 600000, 32)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func (a *App) save(s State) error {
	b, e := json.Marshal(s)
	if e == nil {
		e = atomicWrite(filepath.Join(a.dir, "state.json"), b)
	}
	if e == nil {
		a.state = s
	}
	return e
}
func main() {
	a := &App{dir: env("STATE_DIRECTORY", "/var/lib/easynodedeck"), origin: os.Getenv("PUBLIC_URL"), binary: env("CORE_PATH", "/usr/bin/sing-box"), sni: env("REALITY_SNI", "www.cloudflare.com"), failures: map[string]int{}}
	if len(os.Args) == 2 && os.Args[1] == "init" {
		if _, e := os.Stat(filepath.Join(a.dir, "state.json")); !os.IsNotExist(e) {
			log.Fatal("state already exists or inaccessible")
		}
		pass, err := io.ReadAll(io.LimitReader(os.Stdin, 1025))
		if err != nil || len(pass) < 8 || len(pass) > 1024 {
			log.Fatal("password must contain 8 to 1024 bytes")
		}
		s := State{User: env("ADMIN_USER", "admin"), Salt: random(16), Subs: []Subscription{}}
		s.Hash = digest(string(pass), s.Salt)
		if err := a.save(s); err != nil {
			log.Fatal(err)
		}
		return
	}
	u, err := url.Parse(a.origin)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		log.Fatal("invalid PUBLIC_URL")
	}
	a.host = u.Hostname()
	for i, k := range []string{"SS_PORT", "REALITY_PORT", "HY2_PORT"} {
		v, e := strconv.Atoi(env(k, strconv.Itoa(30443+i)))
		if e != nil || v < 1024 || v > 65535 {
			log.Fatal("invalid ", k)
		}
		a.ports[i] = v
	}
	a.cert = filepath.Join(os.Getenv("CREDENTIALS_DIRECTORY"), "cert.pem")
	a.key = filepath.Join(os.Getenv("CREDENTIALS_DIRECTORY"), "key.pem")
	raw, err := os.ReadFile(filepath.Join(a.dir, "state.json"))
	if err != nil {
		log.Fatal(err)
	}
	if err = json.Unmarshal(raw, &a.state); err != nil {
		log.Fatal(err)
	}
	if err = a.apply(a.state.Node); err != nil {
		log.Print(err)
	}
	go a.monitorRemotes()
	go a.fillMissingNetworks()
	go func() {
		for range time.Tick(30 * time.Second) {
			a.Lock()
			if a.state.Node != nil && !a.core.alive() {
				if e := a.apply(a.state.Node); e != nil {
					log.Print(e)
				}
			}
			a.Unlock()
		}
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", a.api)
	mux.HandleFunc("GET /s/", a.serveSubscription)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/web/", http.StatusFound) })
	mux.Handle("GET /web/", http.FileServer(http.FS(files)))
	mux.Handle("GET /assets/", http.FileServer(http.FS(files)))
	srv := http.Server{Addr: env("LISTEN_ADDR", ":443"), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		mux.ServeHTTP(w, r)
	})}
	log.Fatal(srv.ListenAndServeTLS(a.cert, a.key))
}
func (a *App) serveSubscription(w http.ResponseWriter, r *http.Request) {
	a.Lock()
	defer a.Unlock()
	id := strings.TrimPrefix(r.URL.Path, "/s/")
	for _, s := range a.state.Subs {
		if s.ID == id {
			ua := strings.ToLower(r.UserAgent())
			clash := strings.Contains(ua, "clash") || strings.Contains(ua, "mihomo") || strings.Contains(ua, "stash")
			switch r.URL.Query().Get("format") {
			case "clash":
				clash = true
			case "uri":
				clash = false
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Profile-Update-Interval", "12")
			fmt.Fprint(w, a.subscription(s, clash))
			return
		}
	}
	http.NotFound(w, r)
}

type input struct {
	User, Password, Current, Name, ID string
	Host, Fingerprint                 string
	Port                              int
	Lines                             []string
}

func validName(s string) bool {
	return strings.TrimSpace(s) != "" && len(s) <= 180 && !strings.ContainsAny(s, "\r\n\x00")
}

const sessionLifetime = 12 * time.Hour

func sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: "__Host-deck", Value: value, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: maxAge}
}

// Store only token digests. Publish the cookie after the state write succeeds.
func (a *App) issue(w http.ResponseWriter, s State) error {
	now := time.Now()
	active := make(map[string]time.Time, len(s.Sessions)+1)
	for k, expiry := range s.Sessions {
		if expiry.After(now) {
			active[k] = expiry
		}
	}
	token := random(32)
	active[sessionKey(token)] = now.Add(sessionLifetime)
	s.Sessions = active
	if err := a.save(s); err != nil {
		return err
	}
	http.SetCookie(w, sessionCookie(token, int(sessionLifetime.Seconds())))
	return nil
}
func (a *App) api(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	var in input
	if r.Method == "POST" {
		if !strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]), "application/json") || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != a.origin) {
			http.Error(w, "Invalid origin or content type", 403)
			return
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&in) != nil {
			http.Error(w, "Invalid JSON", 400)
			return
		}
	} else if r.Method != "GET" || path != "state" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	a.Lock()
	defer a.Unlock()
	cookie, _ := r.Cookie("__Host-deck")
	authed := cookie != nil && a.state.Sessions[sessionKey(cookie.Value)].After(time.Now())
	if path == "login" {
		if time.Now().After(a.reset) {
			a.failures = map[string]int{}
			a.reset = time.Now().Add(5 * time.Minute)
		}
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if a.failures[ip] >= 8 {
			http.Error(w, "Too many attempts", 429)
			return
		}
		if in.User != a.state.User || !a.password(in.Password) {
			a.failures[ip]++
			http.Error(w, "Invalid credentials", 401)
			return
		}
		delete(a.failures, ip)
		if err := a.issue(w, a.state); err != nil {
			http.Error(w, "Could not save session", 500)
			return
		}
	} else if !authed {
		http.Error(w, "Sign in", 401)
		return
	}
	if path == "logout" {
		s := a.state
		s.Sessions = maps.Clone(s.Sessions)
		delete(s.Sessions, sessionKey(cookie.Value))
		if err := a.save(s); err != nil {
			http.Error(w, "Could not revoke session", 500)
			return
		}
		http.SetCookie(w, sessionCookie("", -1))
		w.WriteHeader(204)
		return
	}
	refreshID := ""
	explicit := path == "node/network"
	if explicit {
		refreshID = in.ID
		if refreshID == "" {
			http.Error(w, "Missing node ID", 400)
			return
		}
	} else if strings.HasPrefix(path, "remote/") {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Minute))
		info, err := func() (object, error) {
			a.Unlock()
			defer a.Lock()
			return a.manageRemote(path, in)
		}()
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if info != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(info)
			return
		}
		if path == "remote/save" {
			for _, r := range a.state.Remotes {
				if r.Host == in.Host && r.Node.Network == nil {
					refreshID = r.ID
					break
				}
			}
		}
	} else if path == "account" {
		s, err := a.updatedAccount(in)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err = a.issue(w, s); err != nil {
			http.Error(w, "Could not save account", 500)
			return
		}
	} else if path != "state" && path != "login" {
		if err := a.change(path, in); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	if path == "node" && a.state.Node.Network == nil {
		refreshID = "local"
	}
	if refreshID != "" {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(30 * time.Second))
		err := func() error { a.Unlock(); defer a.Lock(); return a.refreshNetwork(r.Context(), refreshID) }()
		if explicit && err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	nodes := []object{}
	if n := a.state.Node; n != nil {
		nodes = append(nodes, object{"id": "local", "name": n.Name, "host": a.host, "local": true, "online": a.core.alive(), "network": n.Network, "lines": n.Lines})
	}
	for _, remote := range a.state.Remotes {
		var online any
		checked := remote.Checked
		probe := "service"
		if time.Since(remote.Checked) < 5*time.Minute {
			online = true
		}
		if result, ok := a.reachability[remote.ID]; ok && result.at.After(checked) {
			online, checked, probe = result.ok, result.at, "tcp"
		}
		nodes = append(nodes, object{"id": remote.ID, "name": remote.Node.Name, "host": remote.Host, "port": remote.Port, "user": "root", "local": false, "online": online, "checked": checked, "probe": probe, "fingerprint": remote.Fingerprint, "network": remote.Node.Network, "lines": remote.Node.Lines})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(object{"user": a.state.User, "nodes": nodes, "subs": a.state.Subs, "host": a.host})
}
func (a *App) password(p string) bool {
	return subtle.ConstantTimeCompare([]byte(a.state.Hash), []byte(digest(p, a.state.Salt))) == 1
}
func (a *App) updatedAccount(in input) (State, error) {
	s := a.state
	if !a.password(in.Current) {
		return s, errors.New("当前密码不正确 / Incorrect password")
	}
	if in.User != "" {
		if !validName(in.User) {
			return s, errors.New("Invalid username")
		}
		s.User = strings.TrimSpace(in.User)
	}
	if in.Password != "" {
		if len(in.Password) < 8 || len(in.Password) > 1024 {
			return s, errors.New("Password must contain 8–1024 bytes")
		}
		s.Salt = random(16)
		s.Hash = digest(in.Password, s.Salt)
	}
	s.Sessions = nil
	return s, nil
}

func (a *App) change(path string, in input) error {
	s := a.state
	s.Subs = append([]Subscription{}, s.Subs...)
	name := strings.TrimSpace(in.Name)
	switch path {
	case "node", "node/delete":
		old := s.Node
		if path == "node/delete" {
			s.Node = nil
		} else {
			if !validName(name) || len(in.Lines) == 0 {
				return errors.New("Choose a name and protocol")
			}
			var n Node
			if old != nil {
				n = *old
			} else {
				created, e := newNode()
				if e != nil {
					return e
				}
				n = *created
			}
			n.Name = name
			n.Lines = nil
			for _, key := range in.Lines {
				i, e := strconv.Atoi(key)
				if e != nil || i < 0 || i >= len(protocols) || slices.Contains(n.Lines, i) {
					return errors.New("Invalid protocol")
				}
				n.Lines = append(n.Lines, i)
			}
			slices.Sort(n.Lines)
			if old != nil && !slices.Equal(old.Lines, n.Lines) {
				n.Network = nil
			}
			s.Node = &n
		}
		for i, sub := range s.Subs {
			var lines = []string{}
			for _, line := range sub.Lines {
				if s.hasLine(line) {
					lines = append(lines, line)
				}
			}
			s.Subs[i].Lines = lines
		}
		changed := s.Node == nil || old == nil || !slices.Equal(s.Node.Lines, old.Lines) || !a.core.alive()
		if changed {
			if err := a.apply(s.Node); err != nil {
				if !a.core.alive() {
					_ = a.apply(old)
				}
				return err
			}
		}
		if err := a.save(s); err != nil {
			if changed {
				_ = a.apply(old)
			}
			return err
		}
		return nil
	case "sub", "sub/delete":
		i := slices.IndexFunc(s.Subs, func(sub Subscription) bool { return sub.ID == in.ID })
		if in.ID != "" && i < 0 {
			return errors.New("Subscription missing")
		}
		if path == "sub/delete" {
			if i < 0 {
				return errors.New("Subscription missing")
			}
			s.Subs = slices.Delete(s.Subs, i, i+1)
			break
		}
		if !validName(name) || len(in.Lines) == 0 {
			return errors.New("Choose a name and connections")
		}
		for _, line := range in.Lines {
			if !s.hasLine(line) {
				return errors.New("Connection missing")
			}
		}
		sub := Subscription{in.ID, name, in.Lines}
		if i < 0 {
			sub.ID = random(24)
			s.Subs = append(s.Subs, sub)
		} else {
			s.Subs[i] = sub
		}
	default:
		return errors.New("Unknown action")
	}
	return a.save(s)
}
func (s State) hasLine(key string) bool {
	id, _, ok := strings.Cut(key, "/")
	if !ok {
		return false
	}
	n := s.Node
	if id != "local" {
		i := slices.IndexFunc(s.Remotes, func(r Remote) bool { return r.ID == id })
		if i < 0 {
			return false
		}
		n = &s.Remotes[i].Node
	}
	if n != nil {
		for _, p := range n.Lines {
			if key == fmt.Sprintf("%s/%d", id, p) {
				return true
			}
		}
	}
	return false
}
