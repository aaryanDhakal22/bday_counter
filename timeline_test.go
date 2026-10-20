package main

import (
	"testing"
	"time"
)

func TestBSToAD(t *testing.T) {
	cases := []struct {
		y, m, d int
		want    string
	}{
		{2075, 1, 1, "2018-04-14"}, // Nepali new year 2075
		{2081, 1, 1, "2024-04-13"}, // Nepali new year 2081
		{2082, 1, 1, "2025-04-14"},
		{2080, 9, 6, "2023-12-22"},
		{2081, 9, 6, "2024-12-21"},
		{2082, 9, 6, "2025-12-21"},
		{2083, 9, 6, "2026-12-21"},
		{2083, 9, 40, "2027-01-14"}, // day clamped to Paush 30
	}
	for _, tc := range cases {
		got, err := bsToAD(tc.y, tc.m, tc.d)
		if err != nil {
			t.Fatalf("%d-%d-%d: %v", tc.y, tc.m, tc.d, err)
		}
		if s := got.Format(time.DateOnly); s != tc.want {
			t.Errorf("%d-%d-%d = %s, want %s", tc.y, tc.m, tc.d, s, tc.want)
		}
	}
	if _, err := bsToAD(2095, 9, 6); err == nil {
		t.Error("expected an error outside the table")
	}
}

func TestADToBSYear(t *testing.T) {
	cases := map[string]int{
		"2026-04-13": 2082,
		"2026-04-14": 2083,
		"2026-10-19": 2083,
		"2026-12-21": 2083,
		"2027-01-10": 2083,
	}
	for ad, want := range cases {
		d, _ := time.Parse(time.DateOnly, ad)
		if got := adToBSYear(d); got != want {
			t.Errorf("adToBSYear(%s) = %d, want %d", ad, got, want)
		}
	}
}

func npt(s string) time.Time {
	loc, _ := time.LoadLocation("Asia/Kathmandu")
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, loc)
	if err != nil {
		panic(err)
	}
	return t
}

func TestMomentStates(t *testing.T) {
	c := defaultConfig() // Paush 6, Asia/Kathmandu, 7 tease days
	cases := []struct {
		now, state, date string
	}{
		{"2026-10-19 12:00:00", "pre", "2026-12-21"},
		{"2026-12-20 23:59:59", "pre", "2026-12-21"},
		{"2026-12-21 00:00:00", "during", "2026-12-21"},
		{"2026-12-21 23:59:59", "during", "2026-12-21"},
		{"2026-12-22 00:00:00", "post", "2026-12-21"},
		{"2026-12-28 23:59:59", "post", "2026-12-21"},
		{"2026-12-29 00:00:00", "pre", "2027-12-21"},
		{"2026-05-01 00:00:00", "pre", "2026-12-21"},
		{"2025-12-25 09:00:00", "post", "2025-12-21"},
		{"2026-01-02 00:00:00", "pre", "2026-12-21"}, // 2025's tease week ended Dec 29
	}
	for _, tc := range cases {
		m := c.momentAt(npt(tc.now))
		if m.State != tc.state || m.Cycle.Date.Format(time.DateOnly) != tc.date {
			t.Errorf("%s: got %s/%s, want %s/%s", tc.now, m.State, m.Cycle.Date.Format(time.DateOnly), tc.state, tc.date)
		}
	}
}

func TestOverrideAndTimezone(t *testing.T) {
	c := defaultConfig()
	c.Overrides[2083] = "2026-12-20"
	m := c.momentAt(npt("2026-12-20 10:00:00"))
	if m.State != "during" || !m.Cycle.Overridden {
		t.Fatalf("override not applied: %+v", m)
	}
	// Midnight in Kathmandu is 18:15 UTC the day before.
	if got := m.Cycle.Start.UTC().Format(time.RFC3339); got != "2026-12-19T18:15:00Z" {
		t.Errorf("start = %s", got)
	}
	c.Timezone = "America/New_York"
	if got := c.cycle(2083).Start.UTC().Format(time.RFC3339); got != "2026-12-20T05:00:00Z" {
		t.Errorf("NY start = %s", got)
	}
}

func TestForcedMoment(t *testing.T) {
	c := defaultConfig()
	real := npt("2026-10-19 12:00:00")
	for _, s := range []string{"pre", "during", "post"} {
		if m := c.forcedMoment(real, s); m.State != s {
			t.Errorf("forced %s gave %s", s, m.State)
		}
	}
	c.TeaseDays = 1
	if m := c.forcedMoment(real, "post"); m.State != "post" {
		t.Errorf("forced post with 1 tease day gave %s", m.State)
	}
}

func TestAge(t *testing.T) {
	c := defaultConfig()
	c.BirthBSYear = 2058
	if a := c.age(c.cycle(2083)); a != 25 {
		t.Errorf("age = %d", a)
	}
}

func TestMissing(t *testing.T) {
	c := defaultConfig()
	now := npt("2026-10-19 12:00:00")
	if len(c.Missing(now)) == 0 {
		t.Fatal("fresh config should have missing fields")
	}
	c.SignOff, c.BirthBSYear = "Me", 2058
	c.Texts["letter.body"], c.Texts["gift.message"] = "hi", "gift"
	if m := c.Missing(now); len(m) != 0 {
		t.Fatalf("unexpected missing: %+v", m)
	}
}
