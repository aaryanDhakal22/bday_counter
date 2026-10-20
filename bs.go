package main

import (
	"fmt"
	"time"
)

// Bikram Sambat (BS) → Gregorian (AD) conversion.
//
// BS months don't follow a formula, so we embed the published month lengths.
// Data: bikram-sambat npm package v1.8.1 (github.com/medic/bikram-sambat, Apache-2.0).
// If a year ever disagrees with the official patro, set an override in /config.

const bsFirstYear = 2050

// bsAnchor is 2050 Baisakh 1 in AD.
var bsAnchor = time.Date(1993, time.April, 13, 0, 0, 0, 0, time.UTC)

var bsMonthLengths = [...][12]int{
	{31, 32, 31, 32, 31, 30, 30, 30, 29, 30, 29, 31}, // 2050
	{31, 31, 31, 32, 31, 31, 30, 29, 30, 29, 30, 30}, // 2051
	{31, 31, 32, 31, 31, 31, 30, 29, 30, 29, 30, 30}, // 2052
	{31, 32, 31, 32, 31, 30, 30, 30, 29, 29, 30, 30}, // 2053
	{31, 32, 31, 32, 31, 30, 30, 30, 29, 30, 29, 31}, // 2054
	{31, 31, 32, 31, 31, 31, 30, 29, 30, 29, 30, 30}, // 2055
	{31, 31, 32, 31, 32, 30, 30, 29, 30, 29, 30, 30}, // 2056
	{31, 32, 31, 32, 31, 30, 30, 30, 29, 29, 30, 31}, // 2057
	{30, 32, 31, 32, 31, 30, 30, 30, 29, 30, 29, 31}, // 2058
	{31, 31, 32, 31, 31, 31, 30, 29, 30, 29, 30, 30}, // 2059
	{31, 31, 32, 32, 31, 30, 30, 29, 30, 29, 30, 30}, // 2060
	{31, 32, 31, 32, 31, 30, 30, 30, 29, 29, 30, 31}, // 2061
	{30, 32, 31, 32, 31, 31, 29, 30, 29, 30, 29, 31}, // 2062
	{31, 31, 32, 31, 31, 31, 30, 29, 30, 29, 30, 30}, // 2063
	{31, 31, 32, 32, 31, 30, 30, 29, 30, 29, 30, 30}, // 2064
	{31, 32, 31, 32, 31, 30, 30, 30, 29, 29, 30, 31}, // 2065
	{31, 31, 31, 32, 31, 31, 29, 30, 30, 29, 29, 31}, // 2066
	{31, 31, 32, 31, 31, 31, 30, 29, 30, 29, 30, 30}, // 2067
	{31, 31, 32, 32, 31, 30, 30, 29, 30, 29, 30, 30}, // 2068
	{31, 32, 31, 32, 31, 30, 30, 30, 29, 29, 30, 31}, // 2069
	{31, 31, 31, 32, 31, 31, 29, 30, 30, 29, 30, 30}, // 2070
	{31, 31, 32, 31, 31, 31, 30, 29, 30, 29, 30, 30}, // 2071
	{31, 32, 31, 32, 31, 30, 30, 29, 30, 29, 30, 30}, // 2072
	{31, 32, 31, 32, 31, 30, 30, 30, 29, 29, 30, 31}, // 2073
	{31, 31, 31, 32, 31, 31, 30, 29, 30, 29, 30, 30}, // 2074
	{31, 31, 32, 31, 31, 31, 30, 29, 30, 29, 30, 30}, // 2075
	{31, 32, 31, 32, 31, 30, 30, 30, 29, 29, 30, 30}, // 2076
	{31, 32, 31, 32, 31, 30, 30, 30, 29, 30, 29, 31}, // 2077
	{31, 31, 31, 32, 31, 31, 30, 29, 30, 29, 30, 30}, // 2078
	{31, 31, 32, 31, 31, 31, 30, 29, 30, 29, 30, 30}, // 2079
	{31, 32, 31, 32, 31, 30, 30, 30, 29, 29, 30, 30}, // 2080
	{31, 32, 31, 32, 31, 30, 30, 30, 29, 30, 29, 31}, // 2081
	{31, 31, 32, 31, 31, 31, 30, 29, 30, 29, 30, 30}, // 2082
	{31, 31, 32, 31, 31, 31, 30, 29, 30, 29, 30, 30}, // 2083
	{31, 31, 32, 31, 31, 30, 30, 30, 29, 30, 30, 30}, // 2084
	{31, 32, 31, 32, 30, 31, 30, 30, 29, 30, 30, 30}, // 2085
	{30, 32, 31, 32, 31, 30, 30, 30, 29, 30, 30, 30}, // 2086
	{31, 31, 32, 31, 31, 31, 30, 30, 29, 30, 30, 30}, // 2087
	{30, 31, 32, 32, 30, 31, 30, 30, 29, 30, 30, 30}, // 2088
	{30, 32, 31, 32, 31, 30, 30, 30, 29, 30, 30, 30}, // 2089
	{30, 32, 31, 32, 31, 30, 30, 30, 29, 30, 30, 30}, // 2090
}

const bsLastYear = bsFirstYear + len(bsMonthLengths) - 1

var bsMonthNames = [12]string{
	"Baisakh", "Jestha", "Asar", "Shrawan", "Bhadra", "Asoj",
	"Kartik", "Mangsir", "Paush", "Magh", "Falgun", "Chaitra",
}

func bsMonthName(m int) string {
	if m < 1 || m > 12 {
		return "?"
	}
	return bsMonthNames[m-1]
}

func bsDaysInMonth(year, month int) (int, error) {
	if year < bsFirstYear || year > bsLastYear {
		return 0, fmt.Errorf("no calendar data for %d BS (have %d–%d)", year, bsFirstYear, bsLastYear)
	}
	if month < 1 || month > 12 {
		return 0, fmt.Errorf("invalid BS month %d", month)
	}
	return bsMonthLengths[year-bsFirstYear][month-1], nil
}

// bsToAD returns the Gregorian calendar date (as UTC midnight) of a BS date.
// A day past the end of the month is clamped to the month's last day.
func bsToAD(year, month, day int) (time.Time, error) {
	dim, err := bsDaysInMonth(year, month)
	if err != nil {
		return time.Time{}, err
	}
	if day < 1 {
		return time.Time{}, fmt.Errorf("invalid BS day %d", day)
	}
	day = min(day, dim)
	days := day - 1
	for y := bsFirstYear; y < year; y++ {
		for _, n := range bsMonthLengths[y-bsFirstYear] {
			days += n
		}
	}
	for m := 1; m < month; m++ {
		days += bsMonthLengths[year-bsFirstYear][m-1]
	}
	return bsAnchor.AddDate(0, 0, days), nil
}

// adToBSYear returns the BS year that a Gregorian date falls in.
func adToBSYear(t time.Time) int {
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	y := t.Year() + 56
	for y+1 <= bsLastYear {
		next, _ := bsToAD(y+1, 1, 1)
		if d.Before(next) {
			break
		}
		y++
	}
	return y
}
