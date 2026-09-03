// Command loggap scans a log file for gaps in time between consecutive
// timestamped lines. A service that hangs, deadlocks, or gets starved of
// CPU right before it crashes often just stops logging for a while - no
// error, no stack trace, just silence. That silence is easy to miss by
// eye in a file with tens of thousands of lines. loggap finds it for you.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
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
	File               string  `json:"file"`
	MinGapSeconds      float64 `json:"min_gap_seconds"`
	LinesScanned       int     `json:"lines_scanned"`
	LinesWithTimestamp int     `json:"lines_with_timestamp"`
	Gaps               []gap   `json:"gaps"`
}

func main() {
	jsonOutput := flag.Bool("json", false, "print results as JSON instead of plain text")
	minGap := flag.Duration("min-gap", 30*time.Second, "smallest gap worth reporting (e.g. 30s, 5m)")
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

	r, err := openInput(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "loggap: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	rep, err := scan(r, path, *minGap)
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
	return f, nil
}

func scan(r io.Reader, path string, minGap time.Duration) (*report, error) {
	rep := &report{
		File:          path,
		MinGapSeconds: minGap.Seconds(),
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

		t, ok := extractTimestamp(line, now)
		if !ok {
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
