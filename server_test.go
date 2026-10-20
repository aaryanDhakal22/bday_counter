package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T, now time.Time) (*server, *httptest.Server, *http.Client) {
	t.Helper()
	dir := t.TempDir()
	cfg, err := loadConfig(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	stats, _ := loadStats(filepath.Join(dir, "stats.json"))
	auth, err := newAuth("hunter2", "", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	web, _ := fs.Sub(webEmbed, "web")
	tmpl, _ := fs.Sub(templatesEmbed, "templates")
	s := &server{cfg: cfg, stats: stats, auth: auth, dataDir: dir, web: web, tmplFS: tmpl, now: func() time.Time { return now }}
	ts := httptest.NewServer(s.routes())
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return s, ts, &http.Client{Jar: jar}
}

func login(t *testing.T, ts *httptest.Server, c *http.Client) {
	t.Helper()
	res, err := c.PostForm(ts.URL+"/config/login", url.Values{"password": {"hunter2"}})
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if p := res.Request.URL.Path; p != "/config/setup" && p != "/config/home" {
		t.Fatalf("after login landed on %s", res.Request.URL)
	}
}

func get(t *testing.T, c *http.Client, u string) (int, string) {
	t.Helper()
	res, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func send(t *testing.T, c *http.Client, method, u string, form url.Values) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(method, u, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

func site(t *testing.T, c *http.Client, u string) siteResp {
	t.Helper()
	_, body := get(t, c, u)
	var r siteResp
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("bad json: %v\n%s", err, body)
	}
	return r
}

func TestAdminFlow(t *testing.T) {
	s, ts, c := newTestServer(t, npt("2026-10-19 12:00:00"))

	if code, _ := get(t, http.DefaultClient, ts.URL+"/config/home"); code != 200 {
		t.Fatalf("unauthenticated /config/home: %d", code)
	}
	if _, body := get(t, http.DefaultClient, ts.URL+"/config/home"); !strings.Contains(body, "password") {
		t.Fatal("expected the login page")
	}
	login(t, ts, c)

	// Wizard: an invalid step re-renders with an error, valid steps advance.
	_, body := send(t, c, "POST", ts.URL+"/config/setup/1", url.Values{"nickname": {"Mutu"}, "signOff": {""}, "birthBSYear": {"2058"}})
	if !strings.Contains(body, "can&#39;t be empty") {
		t.Fatalf("expected a validation error, got:\n%s", body)
	}
	steps := []url.Values{
		{"nickname": {"Mutu"}, "signOff": {"Me"}, "birthBSYear": {"2058"}},
		{"bsMonth": {"9"}, "bsDay": {"6"}, "timezone": {"Asia/Kathmandu"}},
		{"letter": {"Mero pyaro Mutu…"}, "gift": {"Coupon 🍝"}},
		{"envelope.notes": {"a\nb\nc"}, "reasons": {"1\n2\n3\n4\n5\n6"}},
	}
	for i, f := range steps {
		res, body := send(t, c, "POST", ts.URL+"/config/setup/"+string(rune('1'+i)), f)
		next := `hx-post="/config/setup/` + string(rune('2'+i)) + `"`
		if i == len(steps)-1 {
			next = "All set!"
		}
		if res.StatusCode != 200 || !strings.Contains(body, next) {
			t.Fatalf("step %d: %d, want %s in\n%s", i+1, res.StatusCode, next, body)
		}
	}
	cfg := s.cfg.Get()
	if !cfg.SetupDone || cfg.SignOff != "Me" || len(cfg.Lists["reasons"]) != 6 {
		t.Fatalf("wizard didn't save: %+v", cfg)
	}
	if m := cfg.Missing(s.now()); len(m) != 0 {
		t.Fatalf("still missing %+v", m)
	}

	for _, tab := range adminTabs {
		code, body := get(t, c, ts.URL+"/config/"+tab.Key)
		if code != 200 || !strings.Contains(body, "</html>") {
			t.Fatalf("tab %s: %d\n%s", tab.Key, code, body)
		}
	}
	for _, f := range []string{"years", "banners", "status"} {
		if code, _ := get(t, c, ts.URL+"/config/fragment/"+f); code != 200 {
			t.Fatalf("fragment %s: %d", f, code)
		}
	}
	for step := 1; step <= 5; step++ {
		if code, _ := get(t, c, ts.URL+"/config/setup?step="+string(rune('0'+step))); code != 200 {
			t.Fatalf("setup step %d: %d", step, code)
		}
	}
	if code, body := get(t, c, ts.URL+"/config/setup/dates?bsMonth=9&bsDay=6&timezone=Asia/Kathmandu"); code != 200 || !strings.Contains(body, "Dec 21 2026") {
		t.Fatalf("dates preview: %d %s", code, body)
	}

	// Field edits answer with an HX-Trigger toast.
	res, _ := send(t, c, "PUT", ts.URL+"/config/text/pre.title", url.Values{"value": {"Birthday aauna ✨"}})
	if h := res.Header.Get("HX-Trigger"); !strings.Contains(h, `"saved"`) || strings.ContainsFunc(h, func(r rune) bool { return r > 127 }) {
		t.Fatalf("HX-Trigger = %q", h)
	}
	if s.cfg.Get().Texts["pre.title"] != "Birthday aauna ✨" {
		t.Fatal("text not saved")
	}
	res, _ = send(t, c, "PUT", ts.URL+"/config/field/birthBSYear", url.Values{"value": {"1990"}})
	if !strings.Contains(res.Header.Get("HX-Trigger"), "saveError") {
		t.Fatal("expected a validation error toast")
	}
	send(t, c, "PUT", ts.URL+"/config/num/cake.candles", url.Values{"value": {"7"}})
	send(t, c, "PUT", ts.URL+"/config/flag/music", url.Values{})
	if cfg := s.cfg.Get(); cfg.Nums["cake.candles"] != 7 || cfg.Flags["music"] {
		t.Fatalf("num/flag not saved: %v %v", cfg.Nums["cake.candles"], cfg.Flags["music"])
	}

	// Lists: add, move, edit, delete.
	_, body = send(t, c, "POST", ts.URL+"/config/list/reasons", url.Values{"value": {"Timro hasai"}})
	if !strings.Contains(body, `id="list-reasons"`) || !strings.Contains(body, "Timro hasai") {
		t.Fatalf("list add:\n%s", body)
	}
	send(t, c, "POST", ts.URL+"/config/list/reasons/6/move", url.Values{"dir": {"up"}})
	send(t, c, "PUT", ts.URL+"/config/list/reasons/0", url.Values{"value": {"first!"}})
	send(t, c, "DELETE", ts.URL+"/config/list/reasons/1", nil)
	if got := s.cfg.Get().Lists["reasons"]; strings.Join(got, ",") != "first!,3,4,5,Timro hasai,6" {
		t.Fatalf("list ops: %q", got)
	}
	_, body = send(t, c, "POST", ts.URL+"/config/list/envelope.notes", url.Values{"value": {"d"}})
	if !strings.Contains(body, `id="list-envelope-notes"`) {
		t.Fatal("dotted keys need a safe DOM id")
	}

	// Milestones stay sorted.
	send(t, c, "POST", ts.URL+"/config/milestones", url.Values{"count": {"5"}, "text": {"five"}})
	if ms := s.cfg.Get().Milestones; ms[0].Count != 5 {
		t.Fatalf("milestones not sorted: %+v", ms[:2])
	}

	// Override + date fields.
	send(t, c, "PUT", ts.URL+"/config/override/2083", url.Values{"value": {"2026-12-20"}})
	if r := site(t, c, ts.URL+"/api/site"); r.ADDate != "Sun, Dec 20 2026" {
		t.Fatalf("override not used: %s", r.ADDate)
	}
	res, _ = send(t, c, "PUT", ts.URL+"/config/override/2083", url.Values{"value": {"2030-01-01"}})
	if !strings.Contains(res.Header.Get("HX-Trigger"), "saveError") {
		t.Fatal("expected a wrong-year override to be rejected")
	}
	send(t, c, "PUT", ts.URL+"/config/override/2083", url.Values{"value": {""}})

	// Photo upload (a JPEG) and delete.
	var img bytes.Buffer
	m := image.NewRGBA(image.Rect(0, 0, 2000, 1000))
	for i := range m.Pix {
		m.Pix[i] = 200
	}
	m.Set(0, 0, color.Black)
	jpeg.Encode(&img, m, nil)
	var mp bytes.Buffer
	mw := multipart.NewWriter(&mp)
	fw, _ := mw.CreateFormFile("photos", "us.jpg")
	fw.Write(img.Bytes())
	mw.WriteField("caption", "Hamro photo")
	mw.Close()
	req, _ := http.NewRequest("POST", ts.URL+"/config/photos", &mp)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("HX-Request", "true")
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	photos := s.cfg.Get().Photos
	if len(photos) != 1 || photos[0].Caption != "Hamro photo" {
		t.Fatalf("photo not added: %+v", photos)
	}
	f, err := os.Open(filepath.Join(s.photoDir(), photos[0].ID))
	if err != nil {
		t.Fatal(err)
	}
	pc, _, err := image.DecodeConfig(f)
	f.Close()
	if err != nil || pc.Width != photoMaxSide || pc.Height != 800 {
		t.Fatalf("photo not shrunk: %+v %v", pc, err)
	}
	if code, _ := get(t, http.DefaultClient, ts.URL+"/photos/"+photos[0].ID); code != 200 {
		t.Fatalf("photo not served: %d", code)
	}
	if code, _ := get(t, http.DefaultClient, ts.URL+"/photos/..%2fconfig.json"); code != 404 {
		t.Fatalf("path traversal not blocked: %d", code)
	}
	send(t, c, "DELETE", ts.URL+"/config/photos/"+photos[0].ID, nil)
	if len(s.cfg.Get().Photos) != 0 {
		t.Fatal("photo not deleted")
	}

	// Stats: the admin's events aren't counted, hers are.
	http.Post(ts.URL+"/api/event", "application/json", strings.NewReader(`{"counts":{"hug":3,"bogus":9}}`))
	c.Post(ts.URL+"/api/event", "application/json", strings.NewReader(`{"counts":{"hug":100}}`))
	if st := s.stats.Get(); st.Counters["hug"] != 3 || st.Counters["bogus"] != 0 {
		t.Fatalf("stats = %+v", st.Counters)
	}
}

func TestSiteSpoilersAndForcing(t *testing.T) {
	s, ts, admin := newTestServer(t, npt("2026-10-19 12:00:00"))
	s.cfg.Update(func(c *Config) error {
		c.SetupDone, c.SignOff, c.BirthBSYear = true, "Me", 2058
		c.Texts["letter.body"], c.Texts["gift.message"] = "secret letter", "secret gift"
		return nil
	})
	her := http.DefaultClient

	r := site(t, her, ts.URL+"/api/site")
	if r.State != "pre" || r.Text["letter.body"] != "" || r.Text["gift.message"] != "" {
		t.Fatalf("spoilers leaked before the birthday: %s %q", r.State, r.Text["letter.body"])
	}
	if r.Text["envelope.to"] != "Mutu ko lagi 💌" {
		t.Fatalf("placeholder not filled: %q", r.Text["envelope.to"])
	}
	// She can't preview.
	if r := site(t, her, ts.URL+"/api/site?state=during"); r.State != "pre" || r.Mode != "" {
		t.Fatal("preview params must be ignored for her")
	}

	login(t, ts, admin)
	if r := site(t, admin, ts.URL+"/api/site?state=during"); r.State != "during" || r.Text["letter.body"] != "secret letter" || r.Age != 25 || r.Mode != "preview" {
		t.Fatalf("admin preview: %+v", r.State)
	} else if r.Text["during.title"] != "HAPPY BIRTHDAY MUTU" || r.Text["during.age"] != "Aaja timi 25 barsa pugyau 🥹" {
		t.Fatalf("placeholders: %q / %q", r.Text["during.title"], r.Text["during.age"])
	}
	if r := site(t, admin, ts.URL+"/api/site?at=2026-12-23T10:00"); r.State != "post" {
		t.Fatalf("admin at=: %s", r.State)
	}

	// Force for this device only.
	send(t, admin, "POST", ts.URL+"/config/force", url.Values{"scope": {"me"}, "state": {"post"}})
	if r := site(t, admin, ts.URL+"/api/site"); r.State != "post" || r.Mode != "forced-me" {
		t.Fatalf("forced-me: %s %s", r.State, r.Mode)
	}
	if r := site(t, her, ts.URL+"/api/site"); r.State != "pre" {
		t.Fatal("forced-me leaked to her")
	}
	// Force for everyone.
	send(t, admin, "POST", ts.URL+"/config/force", url.Values{"scope": {"everyone"}, "state": {"during"}})
	if r := site(t, her, ts.URL+"/api/site"); r.State != "during" || r.Mode != "forced" {
		t.Fatalf("forced for everyone: %s %s", r.State, r.Mode)
	}
	send(t, admin, "POST", ts.URL+"/config/force", url.Values{"scope": {"everyone"}, "state": {"auto"}})
	if r := site(t, her, ts.URL+"/api/site"); r.State != "pre" {
		t.Fatal("auto didn't restore")
	}

	// Public pages and assets.
	for _, p := range []string{"/", "/style.css", "/js/main.js", "/js/states/during.js", "/assets/pre/1.gif"} {
		if code, _ := get(t, her, ts.URL+p); code != 200 {
			t.Fatalf("%s: %d", p, code)
		}
	}
	if code, _ := get(t, her, ts.URL+"/assets/"); code != 404 {
		t.Fatalf("directory listing: %d", code)
	}
	// Cross-site posts are refused.
	req, _ := http.NewRequest("POST", ts.URL+"/config/force", strings.NewReader("scope=everyone&state=during"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if res, _ := admin.Do(req); res.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site post: %d", res.StatusCode)
	}
}

func TestJPEGOrientation(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	src.Set(0, 0, color.RGBA{255, 0, 0, 255}) // top-left red
	out := orient(src, 6)                     // rotate 90° clockwise
	if b := out.Bounds(); b.Dx() != 2 || b.Dy() != 3 {
		t.Fatalf("bounds %v", b)
	}
	if r, _, _, _ := out.At(1, 0).RGBA(); r>>8 != 255 {
		t.Fatal("red pixel should move to the top-right")
	}
	// Minimal EXIF segment with orientation 6 (big endian).
	exif := []byte("Exif\x00\x00MM\x00\x2a\x00\x00\x00\x08\x00\x01\x01\x12\x00\x03\x00\x00\x00\x01\x00\x06\x00\x00\x00\x00\x00\x00")
	seg := append([]byte{0xFF, 0xE1, 0, byte(len(exif) + 2)}, exif...)
	jpg := append(append([]byte{0xFF, 0xD8}, seg...), 0xFF, 0xDA)
	if o := jpegOrientation(jpg); o != 6 {
		t.Fatalf("orientation = %d", o)
	}
}

func TestProxyAuth(t *testing.T) {
	s, ts, _ := newTestServer(t, npt("2026-10-19 12:00:00"))
	s.auth.password = ""
	asUser := func(user string) (int, string) {
		req, _ := http.NewRequest("GET", ts.URL+"/config/home", nil)
		if user != "" {
			req.Header.Set("Remote-User", user)
		}
		res, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode, res.Header.Get("Set-Cookie")
	}

	// The test client connects from 127.0.0.1.
	s.auth.proxies, _ = parseProxies("10.0.0.0/8")
	if code, _ := asUser("me"); code != http.StatusSeeOther {
		t.Fatalf("Remote-User from an untrusted address was accepted: %d", code)
	}
	s.auth.proxies, _ = parseProxies("127.0.0.1")
	if code, _ := asUser(""); code != http.StatusSeeOther {
		t.Fatalf("no Remote-User should mean no access: %d", code)
	}
	code, cookie := asUser("me")
	if code != 200 || !strings.Contains(cookie, adminCookie) {
		t.Fatalf("trusted Remote-User: %d %q", code, cookie)
	}
	if s.auth.checkPassword("") {
		t.Fatal("an empty password must never log in")
	}
	if _, err := parseProxies("172.19.0.2/32, 10.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseProxies("nope"); err == nil {
		t.Fatal("expected a parse error")
	}
}
