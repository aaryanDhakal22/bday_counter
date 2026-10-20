package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Public JSON API used by the site in web/.

type sitePhoto struct {
	Src     string `json:"src"`
	Caption string `json:"caption"`
}

type siteResp struct {
	Now        int64               `json:"now"`
	State      string              `json:"state"`
	Mode       string              `json:"mode"` // "", preview, forced-me, forced
	Admin      bool                `json:"admin"`
	Start      int64               `json:"start"`
	End        int64               `json:"end"`
	PostEnd    int64               `json:"postEnd"`
	BSYear     int                 `json:"bsYear"`
	BSDate     string              `json:"bsDate"`
	ADDate     string              `json:"adDate"`
	Day        string              `json:"day"`      // local date of "now", e.g. 2026-10-19
	DayIndex   int                 `json:"dayIndex"` // days since 2000-01-01, for rotating notes
	Age        int                 `json:"age"`
	Name       string              `json:"name"`
	From       string              `json:"from"`
	Flags      map[string]bool     `json:"flags"`
	Nums       map[string]float64  `json:"nums"`
	Text       map[string]string   `json:"text"`
	Lists      map[string][]string `json:"lists"`
	Milestones []Milestone         `json:"milestones"`
	Photos     []sitePhoto         `json:"photos"`
}

func isState(s string) bool { return s == "pre" || s == "during" || s == "post" }

// stageVisible keeps birthday surprises out of the API until it's time.
func stageVisible(stage, state string) bool {
	switch stage {
	case stageDuring:
		return state == "during" || state == "post"
	case stagePost:
		return state == "post"
	}
	return true
}

// moment works out which state to show. Only the admin may preview a date or
// force a state for their own browser; a forced state from config applies to everyone.
func (s *server) moment(r *http.Request, c *Config) (m Moment, mode string, admin bool) {
	admin = s.auth.valid(r)
	real := s.now()
	if admin {
		q := r.URL.Query()
		if st := q.Get("state"); isState(st) {
			return c.forcedMoment(real, st), "preview", true
		}
		if at := q.Get("at"); at != "" {
			if t, ok := parseAt(at, c.location()); ok {
				return c.momentAt(t), "preview", true
			}
		}
		if ck, err := r.Cookie(forceCookie); err == nil && isState(ck.Value) {
			return c.forcedMoment(real, ck.Value), "forced-me", true
		}
	}
	if c.ForceState != "" {
		return c.forcedMoment(real, c.ForceState), "forced", admin
	}
	return c.momentAt(real), "", admin
}

func parseAt(s string, loc *time.Location) (time.Time, bool) {
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.UnixMilli(ms), true
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04", time.DateOnly} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (s *server) handleSite(w http.ResponseWriter, r *http.Request) {
	c := s.cfg.Get()
	m, mode, admin := s.moment(r, c)
	age := c.age(m.Cycle)
	from := c.SignOff
	if from == "" {
		from = "Ma" // "Ma le maya garera banako" until you've set your name
	}
	sub := strings.NewReplacer("{name}", c.Nickname, "{NAME}", strings.ToUpper(c.Nickname), "{from}", from, "{age}", strconv.Itoa(age))

	local := m.Now.In(c.location())
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)

	resp := siteResp{
		Now:      m.Now.UnixMilli(),
		State:    m.State,
		Mode:     mode,
		Admin:    admin,
		Start:    m.Cycle.Start.UnixMilli(),
		End:      m.Cycle.End.UnixMilli(),
		PostEnd:  m.Cycle.PostEnd.UnixMilli(),
		BSYear:   m.Cycle.BSYear,
		BSDate:   c.bsDateLabel(m.Cycle.BSYear),
		ADDate:   m.Cycle.Date.Format("Mon, Jan 2 2006"),
		Day:      day.Format(time.DateOnly),
		DayIndex: int(day.Sub(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24),
		Age:      age,
		Name:     c.Nickname,
		From:     c.SignOff,
		Flags:    c.Flags,
		Nums:     c.Nums,
		Text:     map[string]string{},
		Lists:    map[string][]string{},
		Photos:   []sitePhoto{},
	}
	for _, d := range textDefs {
		if stageVisible(d.Stage, m.State) {
			resp.Text[d.Key] = sub.Replace(c.Texts[d.Key])
		}
	}
	for _, d := range listDefs {
		if stageVisible(d.Stage, m.State) {
			items := make([]string, len(c.Lists[d.Key]))
			for i, v := range c.Lists[d.Key] {
				items[i] = sub.Replace(v)
			}
			resp.Lists[d.Key] = items
		}
	}
	for _, ms := range c.Milestones {
		resp.Milestones = append(resp.Milestones, Milestone{ms.Count, sub.Replace(ms.Text)})
	}
	for _, p := range c.Photos {
		resp.Photos = append(resp.Photos, sitePhoto{"/photos/" + p.ID, sub.Replace(p.Caption)})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, resp)
}

func (s *server) handleEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Counts map[string]int `json:"counts"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Your own clicks while logged in to /config don't count.
	if !s.auth.valid(r) {
		if err := s.stats.Add(body.Counts, s.now()); err != nil {
			s.serverError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleWish(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" || utf8.RuneCountInString(text) > 500 {
		http.Error(w, "wish must be 1–500 characters", http.StatusBadRequest)
		return
	}
	c := s.cfg.Get()
	m, _, admin := s.moment(r, c)
	if err := s.stats.AddWish(Wish{Text: text, BSYear: m.Cycle.BSYear, At: s.now(), Test: admin}); err != nil {
		s.serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleRating(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Stars  int    `json:"stars"`
		Review string `json:"review"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	review := strings.TrimSpace(body.Review)
	if body.Stars < 1 || body.Stars > 5 || utf8.RuneCountInString(review) > 300 {
		http.Error(w, "1–5 stars and a review under 300 characters", http.StatusBadRequest)
		return
	}
	c := s.cfg.Get()
	m, _, admin := s.moment(r, c)
	if err := s.stats.AddRating(Rating{Stars: body.Stars, Review: review, BSYear: m.Cycle.BSYear, At: s.now(), Test: admin}); err != nil {
		s.serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handlePhoto(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !photoIDRe.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, filepath.Join(s.photoDir(), name))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
