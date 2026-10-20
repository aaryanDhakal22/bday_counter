package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// /config: a password-protected admin page built with htmx. Every input
// saves itself; responses carry an HX-Trigger header that shows a toast and
// refreshes the "missing fields" banner.

var adminTabs = []struct{ Key, Label string }{
	{"home", "🏠 Home"},
	{"birthday", "🎂 Birthday"},
	{"preview", "👀 Preview"},
	{"settings", "⚙️ Settings"},
	{"text", "✍️ Text & notes"},
	{"photos", "📸 Photos"},
	{"stats", "📊 Stats"},
}

var commonTimezones = []string{
	"Asia/Kathmandu", "Asia/Kolkata", "Asia/Dubai", "Asia/Qatar", "Asia/Tokyo", "Asia/Seoul",
	"Australia/Sydney", "Europe/London", "Europe/Berlin", "America/New_York",
	"America/Chicago", "America/Los_Angeles", "America/Toronto", "UTC",
}

type yearRow struct {
	BSYear                          int
	Label, Computed, Date, Override string
	Weekday                         string
	Overridden, Estimated, Current  bool
}

type textView struct {
	Def   TextDef
	Value string
}

type listView struct {
	Def   ListDef
	Items []string
}

type section struct {
	Name  string
	Texts []textView
	Lists []listView
}

type flagView struct {
	Def FlagDef
	On  bool
}

type flagSection struct {
	Name  string
	Flags []flagView
}

type numView struct {
	Def   NumDef
	Value float64
}

type numSection struct {
	Name string
	Nums []numView
}

type spamRow struct {
	Key, Label        string
	Max, Window, Lock numView
}

type statRow struct {
	Label string
	Value int
}

type previewLink struct {
	Label, Query, Help string
}

type view struct {
	Tab, Title string
	ProxyAuth  bool // signed in through Authelia: no log-out button
	Tabs       []struct{ Key, Label string }
	C          *Config
	Now        time.Time
	Loc        *time.Location
	M          Moment
	Missing    []MissingField
	ForceMe    string
	Years      []yearRow

	Sections     []section
	FlagSections []flagSection
	NumSections  []numSection
	Spam         []spamRow

	Stats    Stats
	StatRows []statRow

	Previews []previewLink

	// wizard
	Step int
	Form map[string]string
	Errs map[string]string

	// fragments
	PhotoErrs []string
	Flash     string
	Dates     []yearRow
}

func (s *server) view(r *http.Request, tab string) *view {
	c := s.cfg.Get()
	now := s.now()
	v := &view{
		Tab:       tab,
		ProxyAuth: s.auth.proxyUser(r) != "",
		Title:     "Birthday config",
		Tabs:      adminTabs,
		C:         c,
		Now:       now,
		Loc:       c.location(),
		M:         c.momentAt(now),
		Missing:   c.Missing(now),
		Years:     yearRows(c, now, 6),
		Stats:     s.stats.Get(),
		Form:      map[string]string{},
		Errs:      map[string]string{},
	}
	v.Dates = v.Years[1:4]
	if ck, err := r.Cookie(forceCookie); err == nil && isState(ck.Value) {
		v.ForceMe = ck.Value
	}

	// Text & notes: texts and lists grouped by section, in registry order.
	var names []string
	for _, d := range textDefs {
		if !slices.Contains(names, d.Section) {
			names = append(names, d.Section)
		}
	}
	for _, d := range listDefs {
		if !slices.Contains(names, d.Section) {
			names = append(names, d.Section)
		}
	}
	for _, name := range names {
		sec := section{Name: name}
		for _, d := range textDefs {
			if d.Section == name {
				sec.Texts = append(sec.Texts, textView{d, c.Texts[d.Key]})
			}
		}
		for _, d := range listDefs {
			if d.Section == name {
				sec.Lists = append(sec.Lists, listView{d, c.Lists[d.Key]})
			}
		}
		v.Sections = append(v.Sections, sec)
	}

	for _, d := range flagDefs {
		if n := len(v.FlagSections); n == 0 || v.FlagSections[n-1].Name != d.Section {
			v.FlagSections = append(v.FlagSections, flagSection{Name: d.Section})
		}
		fs := &v.FlagSections[len(v.FlagSections)-1]
		fs.Flags = append(fs.Flags, flagView{d, c.Flags[d.Key]})
	}
	for _, d := range numDefs {
		if d.Section == "Spam guard" {
			continue
		}
		if n := len(v.NumSections); n == 0 || v.NumSections[n-1].Name != d.Section {
			v.NumSections = append(v.NumSections, numSection{Name: d.Section})
		}
		ns := &v.NumSections[len(v.NumSections)-1]
		ns.Nums = append(ns.Nums, numView{d, c.Nums[d.Key]})
	}
	for _, f := range spamFeatures {
		nv := func(suffix string) numView {
			d, _ := numDef("spam." + f.Key + "." + suffix)
			return numView{d, c.Nums[d.Key]}
		}
		v.Spam = append(v.Spam, spamRow{f.Key, f.Label, nv("max"), nv("window"), nv("lock")})
	}

	for _, d := range statDefs {
		v.StatRows = append(v.StatRows, statRow{d.Label, v.Stats.Counters[d.Key]})
	}
	slices.Reverse(v.Stats.Wishes)
	slices.Reverse(v.Stats.Ratings)

	cy := v.M.Cycle
	ms := func(t time.Time) string { return strconv.FormatInt(t.UnixMilli(), 10) }
	v.Previews = []previewLink{
		{"Right now", "", "Exactly what she sees"},
		{"Countdown", "state=pre", "23 days before"},
		{"10 s before midnight", "at=" + ms(cy.Start.Add(-10*time.Second)), "Watch the countdown turn into the birthday"},
		{"Birthday", "state=during", "Cake, wish, gift, letter, slideshow"},
		{"Birthday ending", "at=" + ms(cy.End.Add(-10*time.Second)), "Birthday → teasing week"},
		{"Teasing week", "state=post", "Hangover, Budhi meter, leftover cake, rating"},
		{"Rollover", "at=" + ms(cy.PostEnd.Add(-10*time.Second)), "Teasing week → next year's countdown"},
	}
	return v
}

func yearRows(c *Config, now time.Time, n int) []yearRow {
	cur := c.momentAt(now).Cycle.BSYear
	var rows []yearRow
	for y := cur - 1; y < cur-1+n; y++ {
		cy := c.cycle(y)
		row := yearRow{
			BSYear:     y,
			Label:      c.bsDateLabel(y),
			Computed:   cy.Computed.Format(time.DateOnly),
			Date:       cy.Date.Format("Mon, Jan 2 2006"),
			Weekday:    cy.Date.Weekday().String(),
			Overridden: cy.Overridden,
			Estimated:  cy.Estimated,
			Current:    y == cur,
		}
		if cy.Overridden {
			row.Override = cy.Date.Format(time.DateOnly)
		}
		rows = append(rows, row)
	}
	return rows
}

// ---- helpers ----

type userError string

func (e userError) Error() string { return string(e) }

// trigger sets HX-Trigger with a toast message (and any extra events).
// Non-ASCII is escaped because header values are read as Latin-1.
func trigger(w http.ResponseWriter, err error, events ...string) {
	t := map[string]any{"missingChanged": true}
	if err != nil {
		var ue userError
		if errors.As(err, &ue) {
			t["saveError"] = ue.Error()
		} else {
			t["saveError"] = "Couldn't save: " + err.Error()
		}
	} else {
		t["saved"] = "Saved ✓"
		for _, e := range events {
			t[e] = true
		}
	}
	w.Header().Set("HX-Trigger", asciiJSON(t))
}

func asciiJSON(v any) string {
	b, _ := json.Marshal(v)
	var sb strings.Builder
	for _, r := range string(b) {
		switch {
		case r < 0x80:
			sb.WriteRune(r)
		case r > 0xFFFF:
			r -= 0x10000
			fmt.Fprintf(&sb, `\u%04x\u%04x`, 0xD800+(r>>10), 0xDC00+(r&0x3FF))
		default:
			fmt.Fprintf(&sb, `\u%04x`, r)
		}
	}
	return sb.String()
}

func (s *server) saveResponse(w http.ResponseWriter, err error, events ...string) {
	trigger(w, err, events...)
	w.WriteHeader(http.StatusOK)
}

func formValue(r *http.Request) string { return r.FormValue("value") }

func checkLen(s string, max int) error {
	if utf8.RuneCountInString(s) > max {
		return userError(fmt.Sprintf("Too long (max %d characters)", max))
	}
	return nil
}

func isHX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

// ---- auth pages ----

func (s *server) requireAdmin(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth.proxyUser(r) != "" {
			// Authelia vouched for this request. The cookie lets previews on the
			// public pages (which aren't behind Authelia) know it's you.
			if !s.auth.valid(r) {
				s.auth.login(w, r)
			}
			next(w, r)
			return
		}
		if !s.auth.valid(r) {
			if isHX(r) {
				w.Header().Set("HX-Redirect", "/config/login")
				return
			}
			http.Redirect(w, r, "/config/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	})
}

type loginView struct {
	Enabled bool
	Proxy   bool // signed in through Authelia, no password form
	Error   string
}

func (s *server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if s.auth.proxyUser(r) != "" {
		s.auth.login(w, r)
		http.Redirect(w, r, "/config", http.StatusSeeOther)
		return
	}
	if s.auth.password == "" && s.auth.proxyMode() {
		s.render(w, "login", loginView{Proxy: true})
		return
	}
	if s.auth.valid(r) {
		http.Redirect(w, r, "/config", http.StatusSeeOther)
		return
	}
	s.render(w, "login", loginView{Enabled: s.auth.enabled()})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.auth.password == "" {
		s.render(w, "login", loginView{Proxy: s.auth.proxyMode()})
		return
	}
	if s.auth.tooManyFailures(r) {
		s.render(w, "login", loginView{Enabled: true, Error: "Too many tries. Wait 15 minutes 🙈"})
		return
	}
	if !s.auth.checkPassword(r.FormValue("password")) {
		s.auth.recordFailure(r)
		time.Sleep(400 * time.Millisecond)
		s.render(w, "login", loginView{Enabled: true, Error: "Wrong password 🙈"})
		return
	}
	s.auth.login(w, r)
	http.Redirect(w, r, "/config", http.StatusSeeOther)
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.logout(w, r)
	http.Redirect(w, r, "/config/login", http.StatusSeeOther)
}

// ---- pages ----

func (s *server) handleConfigRoot(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Get().SetupDone {
		http.Redirect(w, r, "/config/setup", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/config/home", http.StatusSeeOther)
}

func (s *server) handleTab(w http.ResponseWriter, r *http.Request) {
	tab := r.PathValue("tab")
	if !slices.ContainsFunc(adminTabs, func(t struct{ Key, Label string }) bool { return t.Key == tab }) {
		http.NotFound(w, r)
		return
	}
	s.render(w, "page", s.view(r, tab))
}

func (s *server) handleFragment(w http.ResponseWriter, r *http.Request) {
	switch name := r.PathValue("name"); name {
	case "years", "banners", "status":
		s.render(w, name, s.view(r, ""))
	default:
		http.NotFound(w, r)
	}
}

// ---- setup wizard ----

const setupSteps = 4

func (s *server) handleSetup(w http.ResponseWriter, r *http.Request) {
	step, _ := strconv.Atoi(r.URL.Query().Get("step"))
	if step < 1 || step > setupSteps+1 {
		step = 1
	}
	v := s.view(r, "setup")
	v.Step = step
	v.Form = wizardForm(v.C)
	s.renderWizard(w, r, v)
}

func (s *server) renderWizard(w http.ResponseWriter, r *http.Request, v *view) {
	v.Title = "Setup"
	if isHX(r) {
		s.render(w, "wizard", v)
		return
	}
	s.render(w, "setup", v)
}

func wizardForm(c *Config) map[string]string {
	f := map[string]string{
		"nickname":       c.Nickname,
		"signOff":        c.SignOff,
		"bsMonth":        strconv.Itoa(c.BSMonth),
		"bsDay":          strconv.Itoa(c.BSDay),
		"timezone":       c.Timezone,
		"letter":         c.Texts["letter.body"],
		"gift":           c.Texts["gift.message"],
		"envelope.notes": strings.Join(c.Lists["envelope.notes"], "\n"),
		"reasons":        strings.Join(c.Lists["reasons"], "\n"),
	}
	if c.BirthBSYear != 0 {
		f["birthBSYear"] = strconv.Itoa(c.BirthBSYear)
	}
	return f
}

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

var errInvalid = errors.New("invalid")

func (s *server) handleSetupStep(w http.ResponseWriter, r *http.Request) {
	step, _ := strconv.Atoi(r.PathValue("step"))
	if step < 1 || step > setupSteps {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	errs := map[string]string{}
	val := func(k string) string { return strings.TrimSpace(r.PostForm.Get(k)) }
	now := s.now()

	err := s.cfg.Update(func(c *Config) error {
		switch step {
		case 1:
			if err := validName(val("nickname")); err != nil {
				errs["nickname"] = err.Error()
			}
			if err := validName(val("signOff")); err != nil {
				errs["signOff"] = err.Error()
			}
			y, err := strconv.Atoi(val("birthBSYear"))
			if err == nil {
				err = validBirthYear(y, now)
			} else {
				err = errors.New("enter a year like 2058")
			}
			if err != nil {
				errs["birthBSYear"] = err.Error()
			}
			c.Nickname, c.SignOff, c.BirthBSYear = val("nickname"), val("signOff"), y
		case 2:
			m, err1 := strconv.Atoi(val("bsMonth"))
			d, err2 := strconv.Atoi(val("bsDay"))
			if err1 != nil || m < 1 || m > 12 {
				errs["bsMonth"] = "pick a month"
			}
			if err2 != nil || d < 1 || d > 32 {
				errs["bsDay"] = "day should be 1–32"
			}
			if _, err := time.LoadLocation(val("timezone")); err != nil || val("timezone") == "" {
				errs["timezone"] = "unknown timezone (try Asia/Kathmandu)"
			}
			c.BSMonth, c.BSDay, c.Timezone = m, d, val("timezone")
		case 3:
			if val("letter") == "" {
				errs["letter"] = "write her something 💌"
			}
			if val("gift") == "" {
				errs["gift"] = "what's in the gift box?"
			}
			if err := checkLen(val("letter"), 10000); err != nil {
				errs["letter"] = err.Error()
			}
			c.Texts["letter.body"], c.Texts["gift.message"] = val("letter"), val("gift")
		case 4:
			notes, reasons := splitLines(r.PostForm.Get("envelope.notes")), splitLines(r.PostForm.Get("reasons"))
			if len(notes) < 3 {
				errs["envelope.notes"] = fmt.Sprintf("at least 3 notes, one per line (have %d)", len(notes))
			}
			if len(reasons) < 5 {
				errs["reasons"] = fmt.Sprintf("at least 5 reasons, one per line (have %d)", len(reasons))
			}
			c.Lists["envelope.notes"], c.Lists["reasons"] = notes, reasons
			c.SetupDone = true
		}
		if len(errs) > 0 {
			return errInvalid
		}
		return nil
	})

	v := s.view(r, "setup")
	if err != nil {
		if !errors.Is(err, errInvalid) {
			s.serverError(w, err)
			return
		}
		v.Step, v.Errs = step, errs
		v.Form = wizardForm(v.C)
		for k := range r.PostForm {
			v.Form[k] = r.PostForm.Get(k)
		}
		s.renderWizard(w, r, v)
		return
	}
	if !isHX(r) {
		http.Redirect(w, r, fmt.Sprintf("/config/setup?step=%d", step+1), http.StatusSeeOther)
		return
	}
	v.Step = step + 1
	v.Form = wizardForm(v.C)
	s.render(w, "wizard", v)
}

// handleSetupDates previews which AD dates a BS month/day lands on.
func (s *server) handleSetupDates(w http.ResponseWriter, r *http.Request) {
	c := s.cfg.Get()
	if m, err := strconv.Atoi(r.FormValue("bsMonth")); err == nil && m >= 1 && m <= 12 {
		c.BSMonth = m
	}
	if d, err := strconv.Atoi(r.FormValue("bsDay")); err == nil && d >= 1 && d <= 32 {
		c.BSDay = d
	}
	if tz := r.FormValue("timezone"); tz != "" {
		c.Timezone = tz
	}
	v := s.view(r, "setup")
	v.Dates = yearRows(c, s.now(), 4)[1:]
	s.render(w, "date-preview", v)
}

// ---- field updates ----

func (s *server) handleField(w http.ResponseWriter, r *http.Request) {
	name, val := r.PathValue("name"), strings.TrimSpace(formValue(r))
	var events []string
	err := s.cfg.Update(func(c *Config) error {
		switch name {
		case "nickname", "signOff":
			if err := validName(val); err != nil {
				return userError("Name " + err.Error())
			}
			if name == "nickname" {
				c.Nickname = val
			} else {
				c.SignOff = val
			}
		case "birthBSYear":
			y, err := strconv.Atoi(val)
			if err != nil {
				return userError("Enter a BS year like 2058")
			}
			if err := validBirthYear(y, s.now()); err != nil {
				return userError("Birth year " + err.Error())
			}
			c.BirthBSYear = y
		case "bsMonth":
			m, err := strconv.Atoi(val)
			if err != nil || m < 1 || m > 12 {
				return userError("Pick a month")
			}
			c.BSMonth = m
		case "bsDay":
			d, err := strconv.Atoi(val)
			if err != nil || d < 1 || d > 32 {
				return userError("Day should be 1–32")
			}
			c.BSDay = d
		case "timezone":
			if _, err := time.LoadLocation(val); err != nil || val == "" {
				return userError("Unknown timezone (try Asia/Kathmandu)")
			}
			c.Timezone = val
		case "teaseDays":
			d, err := strconv.Atoi(val)
			if err != nil || d < 1 || d > 60 {
				return userError("Teasing days should be 1–60")
			}
			c.TeaseDays = d
		default:
			return userError("Unknown field")
		}
		events = append(events, "datesChanged")
		return nil
	})
	s.saveResponse(w, err, events...)
}

func (s *server) handleOverride(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(r.PathValue("year"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	val := strings.TrimSpace(formValue(r))
	err = s.cfg.Update(func(c *Config) error {
		if val == "" {
			delete(c.Overrides, year)
			return nil
		}
		d, err := time.Parse(time.DateOnly, val)
		if err != nil {
			return userError("Use a date like 2026-12-21")
		}
		if diff := d.Year() - (year - 57); diff < -1 || diff > 1 {
			return userError(fmt.Sprintf("%s doesn't look like it's in %d BS", val, year))
		}
		c.Overrides[year] = val
		return nil
	})
	s.saveResponse(w, err, "datesChanged")
}

func (s *server) handleText(w http.ResponseWriter, r *http.Request) {
	def, ok := textDef(r.PathValue("key"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	val := strings.TrimRight(strings.ReplaceAll(formValue(r), "\r\n", "\n"), " \n")
	err := s.cfg.Update(func(c *Config) error {
		if err := checkLen(val, 10000); err != nil {
			return err
		}
		c.Texts[def.Key] = val
		return nil
	})
	if err == nil && def.Required && strings.TrimSpace(val) == "" {
		err = userError("Saved, but this one is required 💌")
	}
	s.saveResponse(w, err)
}

func (s *server) handleNum(w http.ResponseWriter, r *http.Request) {
	def, ok := numDef(r.PathValue("key"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	err := s.cfg.Update(func(c *Config) error {
		f, err := strconv.ParseFloat(strings.TrimSpace(formValue(r)), 64)
		if err != nil {
			return userError(def.Label + ": enter a number")
		}
		if f < def.Min || f > def.Max {
			return userError(fmt.Sprintf("%s: between %g and %g", def.Label, def.Min, def.Max))
		}
		c.Nums[def.Key] = f
		return nil
	})
	s.saveResponse(w, err)
}

func (s *server) handleFlag(w http.ResponseWriter, r *http.Request) {
	def, ok := flagDef(r.PathValue("key"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	on := formValue(r) == "on"
	err := s.cfg.Update(func(c *Config) error {
		c.Flags[def.Key] = on
		return nil
	})
	s.saveResponse(w, err)
}

// ---- lists ----

func (s *server) renderList(w http.ResponseWriter, def ListDef, err error) {
	trigger(w, err)
	s.render(w, "list", listView{def, s.cfg.Get().Lists[def.Key]})
}

func listIndex(r *http.Request, n int) (int, error) {
	i, err := strconv.Atoi(r.PathValue("i"))
	if err != nil || i < 0 || i >= n {
		return 0, userError("That item moved. Reload the page.")
	}
	return i, nil
}

func (s *server) handleListAdd(w http.ResponseWriter, r *http.Request) {
	def, ok := listDef(r.PathValue("key"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	val := strings.TrimSpace(formValue(r))
	err := s.cfg.Update(func(c *Config) error {
		if val == "" {
			return userError("Write something first ✍️")
		}
		if err := checkLen(val, 2000); err != nil {
			return err
		}
		c.Lists[def.Key] = append(c.Lists[def.Key], val)
		return nil
	})
	s.renderList(w, def, err)
}

func (s *server) handleListEdit(w http.ResponseWriter, r *http.Request) {
	def, ok := listDef(r.PathValue("key"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	val := strings.TrimSpace(formValue(r))
	err := s.cfg.Update(func(c *Config) error {
		i, err := listIndex(r, len(c.Lists[def.Key]))
		if err != nil {
			return err
		}
		if val == "" {
			return userError("Empty: use ✕ to delete it instead")
		}
		if err := checkLen(val, 2000); err != nil {
			return err
		}
		c.Lists[def.Key][i] = val
		return nil
	})
	s.saveResponse(w, err)
}

func (s *server) handleListDelete(w http.ResponseWriter, r *http.Request) {
	def, ok := listDef(r.PathValue("key"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	err := s.cfg.Update(func(c *Config) error {
		i, err := listIndex(r, len(c.Lists[def.Key]))
		if err != nil {
			return err
		}
		c.Lists[def.Key] = slices.Delete(c.Lists[def.Key], i, i+1)
		return nil
	})
	s.renderList(w, def, err)
}

func moveItem[T any](items []T, i int, dir string) {
	j := i - 1
	if dir == "down" {
		j = i + 1
	}
	if j >= 0 && j < len(items) {
		items[i], items[j] = items[j], items[i]
	}
}

func (s *server) handleListMove(w http.ResponseWriter, r *http.Request) {
	def, ok := listDef(r.PathValue("key"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	err := s.cfg.Update(func(c *Config) error {
		i, err := listIndex(r, len(c.Lists[def.Key]))
		if err != nil {
			return err
		}
		moveItem(c.Lists[def.Key], i, r.FormValue("dir"))
		return nil
	})
	s.renderList(w, def, err)
}

// ---- hug milestones ----

func (s *server) renderMilestones(w http.ResponseWriter, r *http.Request, err error) {
	trigger(w, err)
	s.render(w, "milestones", s.view(r, "text"))
}

func parseMilestone(r *http.Request) (Milestone, error) {
	n, err := strconv.Atoi(strings.TrimSpace(r.FormValue("count")))
	if err != nil || n < 1 || n > 1_000_000 {
		return Milestone{}, userError("Hug count should be a number above 0")
	}
	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		return Milestone{}, userError("Write the milestone message")
	}
	if err := checkLen(text, 500); err != nil {
		return Milestone{}, err
	}
	return Milestone{n, text}, nil
}

func (s *server) handleMilestoneAdd(w http.ResponseWriter, r *http.Request) {
	m, err := parseMilestone(r)
	if err == nil {
		err = s.cfg.Update(func(c *Config) error {
			c.Milestones = append(c.Milestones, m)
			return nil
		})
	}
	s.renderMilestones(w, r, err)
}

func (s *server) handleMilestoneEdit(w http.ResponseWriter, r *http.Request) {
	m, err := parseMilestone(r)
	if err == nil {
		err = s.cfg.Update(func(c *Config) error {
			i, err := listIndex(r, len(c.Milestones))
			if err != nil {
				return err
			}
			c.Milestones[i] = m
			return nil
		})
	}
	s.renderMilestones(w, r, err)
}

func (s *server) handleMilestoneDelete(w http.ResponseWriter, r *http.Request) {
	err := s.cfg.Update(func(c *Config) error {
		i, err := listIndex(r, len(c.Milestones))
		if err != nil {
			return err
		}
		c.Milestones = slices.Delete(c.Milestones, i, i+1)
		return nil
	})
	s.renderMilestones(w, r, err)
}

// ---- force state ----

func (s *server) handleForce(w http.ResponseWriter, r *http.Request) {
	scope, state := r.FormValue("scope"), r.FormValue("state")
	if state == "auto" {
		state = ""
	}
	if state != "" && !isState(state) {
		http.Error(w, "bad state", http.StatusBadRequest)
		return
	}
	var err error
	forceMe := ""
	if ck, e := r.Cookie(forceCookie); e == nil && isState(ck.Value) {
		forceMe = ck.Value
	}
	switch scope {
	case "me":
		forceMe = state
		ck := &http.Cookie{Name: forceCookie, Value: state, Path: "/", HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode}
		if state == "" {
			ck.MaxAge = -1
		}
		http.SetCookie(w, ck)
	case "everyone":
		err = s.cfg.Update(func(c *Config) error {
			c.ForceState = state
			return nil
		})
	default:
		http.Error(w, "bad scope", http.StatusBadRequest)
		return
	}
	trigger(w, err, "datesChanged")
	v := s.view(r, "preview")
	v.ForceMe = forceMe
	s.render(w, "force-panel", v)
}

// ---- photos ----

func (s *server) renderPhotos(w http.ResponseWriter, r *http.Request, err error, flash string, photoErrs []string) {
	trigger(w, err)
	v := s.view(r, "photos")
	v.Flash, v.PhotoErrs = flash, photoErrs
	s.render(w, "photos-grid", v)
}

func (s *server) handlePhotoUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 300<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.renderPhotos(w, r, userError("Upload failed: "+err.Error()), "", nil)
		return
	}
	defer r.MultipartForm.RemoveAll()
	caption := strings.TrimSpace(r.FormValue("caption"))
	var added []Photo
	var errs []string
	for _, fh := range r.MultipartForm.File["photos"] {
		f, err := fh.Open()
		if err != nil {
			errs = append(errs, fh.Filename+": "+err.Error())
			continue
		}
		id, err := savePhoto(s.photoDir(), f)
		f.Close()
		if err != nil {
			errs = append(errs, fh.Filename+": "+err.Error())
			continue
		}
		added = append(added, Photo{ID: id, Caption: caption})
	}
	var err error
	if len(added) > 0 {
		err = s.cfg.Update(func(c *Config) error {
			c.Photos = append(c.Photos, added...)
			return nil
		})
	} else if len(errs) == 0 {
		err = userError("Pick at least one photo")
	}
	flash := ""
	if len(added) > 0 {
		flash = fmt.Sprintf("Added %d photo(s) 📸", len(added))
	}
	s.renderPhotos(w, r, err, flash, errs)
}

func photoIndex(c *Config, id string) int {
	return slices.IndexFunc(c.Photos, func(p Photo) bool { return p.ID == id })
}

func (s *server) handlePhotoCaption(w http.ResponseWriter, r *http.Request) {
	id, val := r.PathValue("id"), strings.TrimSpace(formValue(r))
	err := s.cfg.Update(func(c *Config) error {
		i := photoIndex(c, id)
		if i < 0 {
			return userError("That photo is gone. Reload the page.")
		}
		if err := checkLen(val, 300); err != nil {
			return err
		}
		c.Photos[i].Caption = val
		return nil
	})
	s.saveResponse(w, err)
}

func (s *server) handlePhotoDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.cfg.Update(func(c *Config) error {
		i := photoIndex(c, id)
		if i < 0 {
			return userError("That photo is gone. Reload the page.")
		}
		c.Photos = slices.Delete(c.Photos, i, i+1)
		return nil
	})
	if err == nil && photoIDRe.MatchString(id) {
		if e := os.Remove(filepath.Join(s.photoDir(), id)); e != nil && !errors.Is(e, os.ErrNotExist) {
			err = e
		}
	}
	s.renderPhotos(w, r, err, "", nil)
}

func (s *server) handlePhotoMove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.cfg.Update(func(c *Config) error {
		i := photoIndex(c, id)
		if i < 0 {
			return userError("That photo is gone. Reload the page.")
		}
		moveItem(c.Photos, i, r.FormValue("dir"))
		return nil
	})
	s.renderPhotos(w, r, err, "", nil)
}

// ---- stats ----

func (s *server) handleStatsReset(w http.ResponseWriter, r *http.Request) {
	err := s.stats.Reset()
	trigger(w, err)
	s.render(w, "stats-panel", s.view(r, "stats"))
}

// ---- template helpers ----

var templateFuncs = template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"seq": func(n int) []int {
		out := make([]int, n)
		for i := range out {
			out[i] = i + 1
		}
		return out
	},
	"monthName": bsMonthName,
	"mod":       func(a, b int) int { return a % b },
	"domid":     func(s string) string { return strings.ReplaceAll(s, ".", "-") },
	"states":    func() []string { return []string{"auto", "pre", "during", "post"} },
	"months":    func() []string { return bsMonthNames[:] },
	"timezones": func() []string { return commonTimezones },
	"inLoc":     func(t time.Time, loc *time.Location) time.Time { return t.In(loc) },
	"stars": func(n int) string {
		return strings.Repeat("★", n) + strings.Repeat("☆", max(0, 5-n))
	},
	"until": func(from, to time.Time) string { return humanDuration(to.Sub(from)) },
	"since": func(t, now time.Time) string {
		if t.IsZero() {
			return "never"
		}
		return humanDuration(now.Sub(t)) + " ago"
	},
	"stateEmoji": func(s string) string {
		switch s {
		case "pre":
			return "⏳"
		case "during":
			return "🎂"
		case "post":
			return "😌"
		}
		return "✨"
	},
	"stateName": func(s string) string {
		switch s {
		case "pre":
			return "Countdown"
		case "during":
			return "Birthday!"
		case "post":
			return "Teasing week"
		}
		return "Auto"
	},
	"num":   func(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) },
	"lines": func(s string) int { return min(max(strings.Count(s, "\n")+2, 2), 16) },
	"pct": func(n, total int) int {
		if total == 0 {
			return 0
		}
		return n * 100 / total
	},
}

func humanDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	plural := func(n int, unit string) string {
		if n == 1 {
			return "1 " + unit
		}
		return strconv.Itoa(n) + " " + unit + "s"
	}
	switch {
	case days > 0:
		return plural(days, "day") + " " + plural(hours, "hour")
	case hours > 0:
		return plural(hours, "hour") + " " + plural(mins, "minute")
	case mins > 0:
		return plural(mins, "minute")
	}
	return "a few seconds"
}
