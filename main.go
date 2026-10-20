// Command bday_counter serves Mutu's birthday site and its /config admin page.
//
// Environment:
//
//	ADMIN_PASSWORD  password for /config (required to use /config)
//	PORT            listen port (default 8080)
//	DATA_DIR        where config.json, stats.json and photos/ live (default ./data)
//	SESSION_SECRET  optional; otherwise a random one is kept in DATA_DIR
//	DEV=1           serve web/ and templates/ from disk so edits show on reload
package main

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // the distroless image has no zoneinfo
)

//go:embed web
var webEmbed embed.FS

//go:embed templates
var templatesEmbed embed.FS

type server struct {
	cfg     *ConfigStore
	stats   *StatsStore
	auth    *Auth
	dataDir string
	web     fs.FS
	tmplFS  fs.FS
	dev     bool
	now     func() time.Time

	tmplOnce sync.Once
	tmpl     *template.Template
	tmplErr  error
}

func main() {
	dataDir := envOr("DATA_DIR", "data")
	dev := os.Getenv("DEV") == "1"

	cfg, err := loadConfig(filepath.Join(dataDir, "config.json"))
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	stats, err := loadStats(filepath.Join(dataDir, "stats.json"))
	if err != nil {
		log.Fatalf("stats: %v", err)
	}
	auth, err := newAuth(os.Getenv("ADMIN_PASSWORD"), os.Getenv("SESSION_SECRET"), dataDir)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}

	s := &server{cfg: cfg, stats: stats, auth: auth, dataDir: dataDir, dev: dev, now: time.Now}
	if dev {
		s.web, s.tmplFS = os.DirFS("web"), os.DirFS("templates")
	} else {
		s.web, _ = fs.Sub(webEmbed, "web")
		s.tmplFS, _ = fs.Sub(templatesEmbed, "templates")
	}
	if !auth.enabled() {
		log.Print("ADMIN_PASSWORD is not set: /config is locked until you set it")
	}

	addr := ":" + envOr("PORT", "8080")
	log.Printf("💗 serving on %s (data in %s, dev=%v)", addr, dataDir, dev)
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	// Public site
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.Handle("GET /", s.static())
	mux.HandleFunc("GET /photos/{name}", s.handlePhoto)
	mux.HandleFunc("GET /api/site", s.handleSite)
	mux.HandleFunc("POST /api/event", s.handleEvent)
	mux.HandleFunc("POST /api/wish", s.handleWish)
	mux.HandleFunc("POST /api/rating", s.handleRating)

	// Admin
	mux.HandleFunc("GET /config/login", s.handleLoginPage)
	mux.HandleFunc("POST /config/login", s.handleLogin)
	mux.HandleFunc("POST /config/logout", s.handleLogout)
	admin := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, s.requireAdmin(h)) }
	admin("GET /config", s.handleConfigRoot)
	admin("GET /config/{tab}", s.handleTab)
	admin("GET /config/setup", s.handleSetup)
	admin("POST /config/setup/{step}", s.handleSetupStep)
	admin("GET /config/setup/dates", s.handleSetupDates)
	admin("GET /config/fragment/{name}", s.handleFragment)
	admin("PUT /config/field/{name}", s.handleField)
	admin("PUT /config/override/{year}", s.handleOverride)
	admin("PUT /config/text/{key}", s.handleText)
	admin("PUT /config/num/{key}", s.handleNum)
	admin("PUT /config/flag/{key}", s.handleFlag)
	admin("POST /config/list/{key}", s.handleListAdd)
	admin("PUT /config/list/{key}/{i}", s.handleListEdit)
	admin("DELETE /config/list/{key}/{i}", s.handleListDelete)
	admin("POST /config/list/{key}/{i}/move", s.handleListMove)
	admin("POST /config/milestones", s.handleMilestoneAdd)
	admin("PUT /config/milestones/{i}", s.handleMilestoneEdit)
	admin("DELETE /config/milestones/{i}", s.handleMilestoneDelete)
	admin("POST /config/force", s.handleForce)
	admin("POST /config/photos", s.handlePhotoUpload)
	admin("PUT /config/photos/{id}", s.handlePhotoCaption)
	admin("DELETE /config/photos/{id}", s.handlePhotoDelete)
	admin("POST /config/photos/{id}/move", s.handlePhotoMove)
	admin("POST /config/stats/reset", s.handleStatsReset)

	return securityHeaders(mux)
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFileFS(w, r, s.web, "index.html")
}

// static serves web/: GIFs are cached for a day, code is revalidated.
func (s *server) static() http.Handler {
	files := http.FileServerFS(s.web)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") { // no directory listings
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

func (s *server) photoDir() string { return filepath.Join(s.dataDir, "photos") }

func (s *server) templates() (*template.Template, error) {
	parse := func() (*template.Template, error) {
		return template.New("").Funcs(templateFuncs).ParseFS(s.tmplFS, "*.html")
	}
	if s.dev {
		return parse()
	}
	s.tmplOnce.Do(func() { s.tmpl, s.tmplErr = parse() })
	return s.tmpl, s.tmplErr
}

func (s *server) render(w http.ResponseWriter, name string, data any) {
	t, err := s.templates()
	if err != nil {
		s.serverError(w, err)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		s.serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	buf.WriteTo(w)
}

func (s *server) serverError(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	http.Error(w, "something went wrong: "+err.Error(), http.StatusInternalServerError)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		// Block cross-site form posts against the admin and the API.
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "cross-site request blocked", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
