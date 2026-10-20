package main

import (
	"fmt"
	"time"
)

// Cycle is one year's birthday: pre → during (Start..End) → post (End..PostEnd).
type Cycle struct {
	BSYear     int
	Date       time.Time // AD calendar date of the birthday (UTC midnight)
	Computed   time.Time // what the BS table says, before any override
	Start      time.Time // birthday 00:00 in the configured timezone
	End        time.Time // next day 00:00
	PostEnd    time.Time // End + TeaseDays
	Overridden bool
	Estimated  bool // no calendar data for this year; date guessed from the last known year
}

func (c *Config) cycle(bsYear int) Cycle {
	cy := Cycle{BSYear: bsYear}
	computed, err := bsToAD(bsYear, c.BSMonth, c.BSDay)
	if err != nil {
		// Outside the embedded table: reuse the last known year's AD day.
		last, _ := bsToAD(bsLastYear, c.BSMonth, min(c.BSDay, 29))
		computed = last.AddDate(bsYear-bsLastYear, 0, 0)
		cy.Estimated = true
	}
	cy.Computed, cy.Date = computed, computed
	if s, ok := c.Overrides[bsYear]; ok {
		if d, err := time.Parse(time.DateOnly, s); err == nil {
			cy.Date, cy.Overridden, cy.Estimated = d, true, false
		}
	}
	loc := c.location()
	y, m, d := cy.Date.Date()
	cy.Start = time.Date(y, m, d, 0, 0, 0, 0, loc)
	cy.End = time.Date(y, m, d+1, 0, 0, 0, 0, loc)
	cy.PostEnd = time.Date(y, m, d+1+c.TeaseDays, 0, 0, 0, 0, loc)
	return cy
}

// Moment is the site's state at a point in time.
type Moment struct {
	State string // pre, during, post
	Now   time.Time
	Cycle Cycle
}

func (c *Config) momentAt(now time.Time) Moment {
	y := adToBSYear(now.In(c.location())) - 1
	var cy Cycle
	for i := range 4 {
		cy = c.cycle(y + i)
		if now.Before(cy.PostEnd) {
			break
		}
	}
	state := "post"
	switch {
	case now.Before(cy.Start):
		state = "pre"
	case now.Before(cy.End):
		state = "during"
	}
	return Moment{State: state, Now: now, Cycle: cy}
}

// forcedMoment makes up a "now" inside the requested state of the current
// cycle, so forced/preview pages still show sensible timers.
func (c *Config) forcedMoment(real time.Time, state string) Moment {
	cy := c.momentAt(real).Cycle
	var now time.Time
	switch state {
	case "pre":
		now = cy.Start.Add(-(23*24*time.Hour + 4*time.Hour + 5*time.Minute + 6*time.Second))
	case "during":
		now = cy.Start.Add(2*time.Hour + 21*time.Minute)
	case "post":
		now = cy.End.Add(min(27*time.Hour+13*time.Minute, cy.PostEnd.Sub(cy.End)/2))
	default:
		return c.momentAt(real)
	}
	return c.momentAt(now)
}

// age is how old she turns on the cycle's birthday (0 if unknown).
func (c *Config) age(cy Cycle) int {
	if c.BirthBSYear == 0 || cy.BSYear <= c.BirthBSYear {
		return 0
	}
	return cy.BSYear - c.BirthBSYear
}

func (c *Config) bsDateLabel(bsYear int) string {
	return fmt.Sprintf("%s %d, %d BS", bsMonthName(c.BSMonth), c.BSDay, bsYear)
}
