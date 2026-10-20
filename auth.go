package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	adminCookie = "bday_admin"
	forceCookie = "bday_force" // per-browser forced state, honored only for the admin
	sessionTTL  = 30 * 24 * time.Hour
)

// Auth guards /config with a single password from ADMIN_PASSWORD, or with a
// reverse proxy like Authelia: Remote-User is trusted only from the
// TRUSTED_PROXIES ranges. Sessions are HMAC-signed cookies; changing the
// password logs every session out.
type Auth struct {
	password string
	secret   []byte
	proxies  []netip.Prefix

	mu    sync.Mutex
	fails map[string][]time.Time
}

func newAuth(password, secret, dataDir string, proxies []netip.Prefix) (*Auth, error) {
	a := &Auth{password: password, proxies: proxies, fails: map[string][]time.Time{}}
	if secret != "" {
		a.secret = []byte(secret)
		return a, nil
	}
	// Persist a random secret so sessions survive restarts.
	path := filepath.Join(dataDir, ".session_secret")
	if b, err := os.ReadFile(path); err == nil && len(b) >= 32 {
		a.secret = b
		return a, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	sec := []byte(hex.EncodeToString(b))
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, sec, 0o600); err != nil {
		return nil, err
	}
	a.secret = sec
	return a, nil
}

func (a *Auth) enabled() bool { return a.password != "" || a.proxyMode() }

func (a *Auth) proxyMode() bool { return len(a.proxies) > 0 }

// proxyUser returns the Remote-User the proxy vouched for, or "" if the
// request didn't come from a trusted proxy.
func (a *Auth) proxyUser(r *http.Request) string {
	u := strings.TrimSpace(r.Header.Get("Remote-User"))
	if u == "" || !a.proxyMode() {
		return ""
	}
	addr, err := netip.ParseAddr(clientIP(r))
	if err != nil {
		return ""
	}
	for _, p := range a.proxies {
		if p.Contains(addr.Unmap()) {
			return u
		}
	}
	return ""
}

// parseProxies reads a comma-separated list of CIDRs or bare IPs.
func parseProxies(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		if !strings.Contains(f, "/") {
			addr, err := netip.ParseAddr(f)
			if err != nil {
				return nil, err
			}
			f = netip.PrefixFrom(addr, addr.BitLen()).String()
		}
		p, err := netip.ParsePrefix(f)
		if err != nil {
			return nil, err
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

func (a *Auth) checkPassword(pw string) bool {
	if a.password == "" {
		return false
	}
	x := sha256.Sum256([]byte(pw))
	y := sha256.Sum256([]byte(a.password))
	return subtle.ConstantTimeCompare(x[:], y[:]) == 1
}

func (a *Auth) sign(exp int64) string {
	pw := sha256.Sum256([]byte(a.password))
	m := hmac.New(sha256.New, a.secret)
	m.Write([]byte("admin|" + strconv.FormatInt(exp, 10) + "|"))
	m.Write(pw[:])
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (a *Auth) login(w http.ResponseWriter, r *http.Request) {
	exp := time.Now().Add(sessionTTL).Unix()
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookie,
		Value:    strconv.FormatInt(exp, 10) + "." + a.sign(exp),
		Path:     "/",
		Expires:  time.Unix(exp, 0),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *Auth) logout(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{adminCookie, forceCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode})
	}
}

func (a *Auth) valid(r *http.Request) bool {
	if !a.enabled() {
		return false
	}
	c, err := r.Cookie(adminCookie)
	if err != nil {
		return false
	}
	expStr, sig, ok := strings.Cut(c.Value, ".")
	if !ok {
		return false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(a.sign(exp)))
}

// tooManyFailures rate-limits password guesses per client IP.
func (a *Auth) tooManyFailures(r *http.Request) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	ip := clientIP(r)
	cutoff := time.Now().Add(-15 * time.Minute)
	recent := a.fails[ip][:0]
	for _, t := range a.fails[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	a.fails[ip] = recent
	return len(recent) >= 10
}

func (a *Auth) recordFailure(r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ip := clientIP(r)
	a.fails[ip] = append(a.fails[ip], time.Now())
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
