package main

import (
	"fmt"
	"regexp"
	"time"
)

// tsFormat pairs a regex that matches a timestamp at the start of a log
// line with the Go time layout needed to parse whatever it matches.
type tsFormat struct {
	name string
	re   *regexp.Regexp
	// layout is the Go reference layout for the matched text. If
	// needsYear is set, layout describes the match only; the caller
	// prepends "2006 " and a guessed year before parsing.
	layout    string
	needsYear bool
}

var tsFormats = []tsFormat{
	{
		// 2024-01-02T15:04:05Z or 2024-01-02T15:04:05.123456-07:00
		// RFC3339Nano's ".999999999" is optional in Go's parser, so
		// this layout also matches timestamps with no fraction.
		name:   "rfc3339",
		re:     regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})`),
		layout: time.RFC3339Nano,
	},
	{
		// 02/Jan/2024:15:04:05 -0700 (Apache/nginx access log)
		name:   "clf",
		re:     regexp.MustCompile(`^\d{2}/[A-Za-z]{3}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4}`),
		layout: "02/Jan/2006:15:04:05 -0700",
	},
	{
		// Jan  2 15:04:05 (classic syslog, no year in the line)
		name:      "syslog",
		re:        regexp.MustCompile(`^[A-Za-z]{3}\s+\d{1,2} \d{2}:\d{2}:\d{2}`),
		layout:    "Jan _2 15:04:05",
		needsYear: true,
	},
}

// extractTimestamp looks for a timestamp at the start of line, trying each
// known format in turn. now is used to fill in the year for formats (like
// syslog) that don't carry one, and to catch the case where a log from
// late last year is being read early this year.
func extractTimestamp(line string, now time.Time) (time.Time, bool) {
	for _, f := range tsFormats {
		match := f.re.FindString(line)
		if match == "" {
			continue
		}
		if !f.needsYear {
			t, err := time.Parse(f.layout, match)
			if err == nil {
				return t, true
			}
			continue
		}
		guess := fmt.Sprintf("%d %s", now.Year(), match)
		t, err := time.Parse("2006 "+f.layout, guess)
		if err != nil {
			continue
		}
		// If parsing with the current year puts the timestamp more
		// than a day in the future, the log almost certainly rolled
		// over a year boundary (e.g. reading a December log in
		// January) - back it up one year.
		if t.After(now.Add(24 * time.Hour)) {
			t = t.AddDate(-1, 0, 0)
		}
		return t, true
	}
	return time.Time{}, false
}
