package main

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"slices"
	"sync"
	"time"
)

// Stats collects the fun numbers shown on /config → Stats.
type Stats struct {
	Counters  map[string]int `json:"counters"`
	Wishes    []Wish         `json:"wishes"`
	Ratings   []Rating       `json:"ratings"`
	LastVisit time.Time      `json:"lastVisit"`
}

type Wish struct {
	Text   string    `json:"text"`
	BSYear int       `json:"bsYear"`
	At     time.Time `json:"at"`
	Test   bool      `json:"test,omitempty"` // sent while logged in to /config
}

type Rating struct {
	Stars  int       `json:"stars"`
	Review string    `json:"review"`
	BSYear int       `json:"bsYear"`
	At     time.Time `json:"at"`
	Test   bool      `json:"test,omitempty"`
}

// Event keys the public site may send, with their /config labels.
var statDefs = []struct{ Key, Label string }{
	{"visit", "Visits"},
	{"envelope", "Envelopes opened"},
	{"hearts", "Hearts tapped"},
	{"hug", "Hugs"},
	{"bear", "Bear pets"},
	{"grumpy", "Times the bear got grumpy"},
	{"no", "No-button dodges"},
	{"yes", "Said yes"},
	{"jar", "Reasons pulled from the jar"},
	{"photo", "Photos opened"},
	{"cake_mic", "Candles blown out with the mic"},
	{"cake_tap", "Candles tapped out"},
	{"gift", "Gifts unwrapped"},
	{"letter", "Letters read to the end"},
	{"sweep", "Confetti swept"},
	{"budhi", "Budhi meter steps"},
	{"slice", "Cake slices eaten"},
}

func isStatKey(k string) bool {
	return slices.ContainsFunc(statDefs, func(d struct{ Key, Label string }) bool { return d.Key == k })
}

type StatsStore struct {
	mu   sync.Mutex
	path string
	s    *Stats
}

func loadStats(path string) (*StatsStore, error) {
	st := &StatsStore{path: path, s: &Stats{Counters: map[string]int{}}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, st.s); err != nil {
		return nil, err
	}
	if st.s.Counters == nil {
		st.s.Counters = map[string]int{}
	}
	return st, nil
}

func (st *StatsStore) Get() Stats {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := *st.s
	out.Counters = maps.Clone(st.s.Counters)
	out.Wishes = slices.Clone(st.s.Wishes)
	out.Ratings = slices.Clone(st.s.Ratings)
	return out
}

func (st *StatsStore) update(fn func(s *Stats)) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	fn(st.s)
	return writeJSONAtomic(st.path, st.s)
}

func (st *StatsStore) Add(counts map[string]int, now time.Time) error {
	return st.update(func(s *Stats) {
		for k, n := range counts {
			if isStatKey(k) && n > 0 {
				s.Counters[k] += min(n, 500)
			}
		}
		s.LastVisit = now
	})
}

func (st *StatsStore) AddWish(w Wish) error {
	return st.update(func(s *Stats) {
		s.Wishes = append(s.Wishes, w)
		if len(s.Wishes) > 500 {
			s.Wishes = s.Wishes[len(s.Wishes)-500:]
		}
	})
}

func (st *StatsStore) AddRating(r Rating) error {
	return st.update(func(s *Stats) {
		s.Ratings = append(s.Ratings, r)
		if len(s.Ratings) > 500 {
			s.Ratings = s.Ratings[len(s.Ratings)-500:]
		}
	})
}

func (st *StatsStore) Reset() error {
	return st.update(func(s *Stats) { *s = Stats{Counters: map[string]int{}} })
}
