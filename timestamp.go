package main

import (
	"fmt"
	"regexp"
	"strings"
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
// known format in turn, then falling back to customLayout (if set) for logs
// that don't match any built-in format. now is used to fill in the year for
// formats (like syslog) that don't carry one, and to catch the case where a
// log from late last year is being read early this year.
func extractTimestamp(line string, now time.Time, customLayout string) (time.Time, bool) {
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
		return rollBackIfFuture(t, now), true
	}
	if customLayout != "" {
		return tryCustomFormat(line, customLayout, now)
	}
	return time.Time{}, false
}

// tryCustomFormat parses a timestamp at the start of line using a
// user-supplied Go reference layout. Unlike the built-in formats, we don't
// have a regexp to say exactly how many characters the timestamp occupies,
// so we try a range of prefix lengths around the layout's own length - wide
// enough to absorb the width differences that come from things like
// single- vs double-digit hours or short vs long month names.
func tryCustomFormat(line, layout string, now time.Time) (time.Time, bool) {
	const slack = 10

	minLen := len(layout) - slack
	if minLen < 1 {
		minLen = 1
	}
	maxLen := len(layout) + slack
	if maxLen > len(line) {
		maxLen = len(line)
	}

	for l := minLen; l <= maxLen; l++ {
		t, err := time.Parse(layout, line[:l])
		if err != nil {
			continue
		}
		if !layoutHasYear(layout) {
			t = time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
			t = rollBackIfFuture(t, now)
		}
		return t, true
	}
	return time.Time{}, false
}

// layoutHasYear reports whether a Go reference layout includes a year field
// (either the four-digit "2006" or two-digit "06").
func layoutHasYear(layout string) bool {
	return strings.Contains(layout, "2006") || strings.Contains(layout, "06")
}

// rollBackIfFuture backs a parsed timestamp up by one year if it lands more
// than a day ahead of now. This corrects year-less formats (syslog, or a
// custom layout with no year) for the case where a log from late last year
// is being read early this year.
func rollBackIfFuture(t, now time.Time) time.Time {
	if t.After(now.Add(24 * time.Hour)) {
		return t.AddDate(-1, 0, 0)
	}
	return t
}
