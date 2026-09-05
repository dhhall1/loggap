package main

import (
	"testing"
	"time"
)

func TestExtractTimestamp(t *testing.T) {
	// Fixed "now" so the syslog year-guessing logic is deterministic.
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		line string
		want time.Time
		ok   bool
	}{
		{
			name: "rfc3339 with Z",
			line: "2024-03-01T09:00:00Z INFO starting up",
			want: time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC),
			ok:   true,
		},
		{
			name: "rfc3339 with fraction and numeric offset",
			line: "2024-03-01T09:00:00.123456-07:00 INFO starting up",
			want: time.Date(2024, 3, 1, 9, 0, 0, 123456000, time.FixedZone("", -7*3600)),
			ok:   true,
		},
		{
			name: "apache/nginx combined log format",
			line: `01/Mar/2024:09:00:00 -0700 "GET / HTTP/1.1" 200`,
			want: time.Date(2024, 3, 1, 9, 0, 0, 0, time.FixedZone("", -7*3600)),
			ok:   true,
		},
		{
			name: "classic syslog, current year",
			line: "Jan  2 15:04:05 host myservice[123]: message",
			want: time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC),
			ok:   true,
		},
		{
			name: "classic syslog, day padded with two digits",
			line: "Mar 15 08:00:00 host myservice[123]: message",
			want: time.Date(2024, 3, 15, 8, 0, 0, 0, time.UTC),
			ok:   true,
		},
		{
			name: "syslog line from December read in January rolls back a year",
			line: "Dec 31 23:59:59 host myservice[123]: message",
			want: time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
			ok:   true,
		},
		{
			name: "unrecognized format",
			line: "not a timestamp at all",
			ok:   false,
		},
		{
			name: "empty line",
			line: "",
			ok:   false,
		},
		{
			name: "timestamp not at start of line is ignored",
			line: "INFO 2024-03-01T09:00:00Z starting up",
			ok:   false,
		},
	}

	// The December-rollover case needs a "now" in early January to trigger
	// the correction; every other case uses the shared now above.
	earlyJanNow := time.Date(2024, 1, 2, 6, 0, 0, 0, time.UTC)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testNow := now
			if c.name == "syslog line from December read in January rolls back a year" {
				testNow = earlyJanNow
			}

			got, ok := extractTimestamp(c.line, testNow)
			if ok != c.ok {
				t.Fatalf("extractTimestamp(%q) ok = %v, want %v", c.line, ok, c.ok)
			}
			if ok && !got.Equal(c.want) {
				t.Errorf("extractTimestamp(%q) = %v, want %v", c.line, got, c.want)
			}
		})
	}
}
