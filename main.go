// Command loggap scans a log file for gaps in time between consecutive
// timestamped lines. A service that hangs, deadlocks, or gets starved of
// CPU right before it crashes often just stops logging for a while - no
// error, no stack trace, just silence. That silence is easy to miss by
// eye in a file with tens of thousands of lines. loggap finds it for you.
package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type gap struct {
	FromLine int       `json:"from_line"`
	ToLine   int       `json:"to_line"`
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Seconds  float64   `json:"seconds"`
}

type report struct {
	File               string       `json:"file"`
	MinGapSeconds      float64      `json:"min_gap_seconds"`
	Since              *time.Time   `json:"since,omitempty"`
	Until              *time.Time   `json:"until,omitempty"`
	LinesScanned       int          `json:"lines_scanned"`
	LinesWithTimestamp int          `json:"lines_with_timestamp"`
	Gaps               []gap        `json:"gaps"`
	Histogram          []histBucket `json:"histogram"`
}

// timeFlagLayouts are tried in order when parsing --since/--until. RFC3339
// covers anything loggap itself would print; the rest are for typing a
// window on the command line by hand without fussing over an offset.
var timeFlagLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parseTimeFlag(s string) (time.Time, error) {
	for _, layout := range timeFlagLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("could not parse %q as a time (try RFC3339, e.g. 2024-03-01T09:00:00Z, or 2024-03-01)", s)
}

func main() {
	jsonOutput := flag.Bool("json", false, "print results as JSON instead of plain text")
	minGap := flag.Duration("min-gap", 30*time.Second, "smallest gap worth reporting (e.g. 30s, 5m)")
	format := flag.String("format", "", "Go reference layout for logs that don't match a built-in timestamp format (e.g. \"2006-01-02 15:04:05\")")
	since := flag.String("since", "", "ignore lines timestamped before this time (RFC3339, e.g. 2024-03-01T09:00:00Z, or 2024-03-01)")
	until := flag.String("until", "", "ignore lines timestamped after this time (same formats as --since)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: loggap [flags] [file]\n\n")
		fmt.Fprintf(os.Stderr, "Find gaps in time between consecutive timestamped log lines.\n")
		fmt.Fprintf(os.Stderr, "Reads from stdin if no file is given.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	path := "-"
	if flag.NArg() > 0 {
		path = flag.Arg(0)
	}

	var sinceTime, untilTime *time.Time
	if *since != "" {
		t, err := parseTimeFlag(*since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "loggap: --since: %v\n", err)
			os.Exit(1)
		}
		sinceTime = &t
	}
	if *until != "" {
		t, err := parseTimeFlag(*until)
		if err != nil {
			fmt.Fprintf(os.Stderr, "loggap: --until: %v\n", err)
			os.Exit(1)
		}
		untilTime = &t
	}
	if sinceTime != nil && untilTime != nil && sinceTime.After(*untilTime) {
		fmt.Fprintf(os.Stderr, "loggap: --since (%s) is after --until (%s)\n", sinceTime.Format(time.RFC3339), untilTime.Format(time.RFC3339))
		os.Exit(1)
	}

	r, err := openInput(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "loggap: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	rep, err := scan(r, path, *minGap, *format, sinceTime, untilTime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "loggap: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		printJSON(rep)
	} else {
		printText(rep)
	}
}

func openInput(path string) (io.ReadCloser, error) {
	if path == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("opening %s: %w", path, err)
		}
		return &gzipFile{gz: gz, f: f}, nil
	}
	return f, nil
}

// gzipFile wraps a gzip.Reader together with the underlying file so Close
// tears down both - closing just the gzip.Reader leaves the file descriptor
// open.
type gzipFile struct {
	gz *gzip.Reader
	f  *os.File
}

func (g *gzipFile) Read(p []byte) (int, error) { return g.gz.Read(p) }

func (g *gzipFile) Close() error {
	if err := g.gz.Close(); err != nil {
		g.f.Close()
		return err
	}
	return g.f.Close()
}

func scan(r io.Reader, path string, minGap time.Duration, customLayout string, since, until *time.Time) (*report, error) {
	rep := &report{
		File:          path,
		MinGapSeconds: minGap.Seconds(),
		Since:         since,
		Until:         until,
		Gaps:          []gap{},
	}

	now := time.Now()
	scanner := bufio.NewScanner(r)
	// Log lines are occasionally much longer than bufio's 64KB default
	// (stack traces, JSON blobs); give ourselves plenty of headroom.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		havePrev bool
		prevTime time.Time
		prevLine int
	)

	for scanner.Scan() {
		rep.LinesScanned++
		line := scanner.Text()

		t, ok := extractTimestamp(line, now, customLayout)
		if !ok {
			continue
		}
		if since != nil && t.Before(*since) {
			continue
		}
		if until != nil && t.After(*until) {
			continue
		}
		rep.LinesWithTimestamp++

		if havePrev {
			diff := t.Sub(prevTime)
			if diff >= minGap {
				rep.Gaps = append(rep.Gaps, gap{
					FromLine: prevLine,
					ToLine:   rep.LinesScanned,
					From:     prevTime,
					To:       t,
					Seconds:  diff.Seconds(),
				})
			}
		}

		prevTime = t
		prevLine = rep.LinesScanned
		havePrev = true
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	rep.Histogram = buildHistogram(rep.Gaps, minGap)
	return rep, nil
}

func printJSON(rep *report) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	// Encoding a fixed, in-memory struct to stdout; the error path only
	// fires if stdout itself is broken, which we can't do anything about.
	_ = enc.Encode(rep)
}

func printText(rep *report) {
	if rep.Since != nil || rep.Until != nil {
		fmt.Printf("window: %s -> %s\n", formatWindowBound(rep.Since), formatWindowBound(rep.Until))
	}

	if len(rep.Gaps) == 0 {
		fmt.Printf("%s: no gaps >= %s across %d timestamped lines (of %d scanned)\n",
			rep.File, time.Duration(rep.MinGapSeconds*float64(time.Second)), rep.LinesWithTimestamp, rep.LinesScanned)
		return
	}

	for _, g := range rep.Gaps {
		fmt.Printf("line %d -> %d: %s silence (%s -> %s)\n",
			g.FromLine, g.ToLine,
			time.Duration(g.Seconds*float64(time.Second)),
			g.From.Format(time.RFC3339),
			g.To.Format(time.RFC3339))
	}
	fmt.Printf("\n%d gap(s) >= %s, %d/%d lines had a recognized timestamp\n",
		len(rep.Gaps), time.Duration(rep.MinGapSeconds*float64(time.Second)),
		rep.LinesWithTimestamp, rep.LinesScanned)
}

func formatWindowBound(t *time.Time) string {
	if t == nil {
		return "(none)"
	}
	return t.Format(time.RFC3339)
}
