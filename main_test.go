package main

import (
	"os"
	"testing"
	"time"
)

func TestScanServiceLog(t *testing.T) {
	f, err := os.Open("testdata/service.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	rep, err := scan(f, "testdata/service.log", 30*time.Second, "")
	if err != nil {
		t.Fatal(err)
	}

	if rep.LinesScanned != 5 {
		t.Errorf("LinesScanned = %d, want 5", rep.LinesScanned)
	}
	if rep.LinesWithTimestamp != 5 {
		t.Errorf("LinesWithTimestamp = %d, want 5", rep.LinesWithTimestamp)
	}
	if len(rep.Gaps) != 1 {
		t.Fatalf("len(Gaps) = %d, want 1", len(rep.Gaps))
	}

	g := rep.Gaps[0]
	if g.FromLine != 3 || g.ToLine != 4 {
		t.Errorf("gap lines = %d -> %d, want 3 -> 4", g.FromLine, g.ToLine)
	}
	if g.Seconds != 895 {
		t.Errorf("gap seconds = %v, want 895", g.Seconds)
	}
}

func TestScanNoGaps(t *testing.T) {
	f, err := os.Open("testdata/no_gaps.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	rep, err := scan(f, "testdata/no_gaps.log", 30*time.Second, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(rep.Gaps) != 0 {
		t.Errorf("len(Gaps) = %d, want 0", len(rep.Gaps))
	}
	if rep.LinesWithTimestamp != 3 {
		t.Errorf("LinesWithTimestamp = %d, want 3", rep.LinesWithTimestamp)
	}
}

func TestScanMixedRecognizedAndUnrecognizedLines(t *testing.T) {
	f, err := os.Open("testdata/mixed.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	rep, err := scan(f, "testdata/mixed.log", 30*time.Second, "")
	if err != nil {
		t.Fatal(err)
	}

	if rep.LinesScanned != 5 {
		t.Errorf("LinesScanned = %d, want 5", rep.LinesScanned)
	}
	if rep.LinesWithTimestamp != 3 {
		t.Errorf("LinesWithTimestamp = %d, want 3", rep.LinesWithTimestamp)
	}
	if len(rep.Gaps) != 1 {
		t.Fatalf("len(Gaps) = %d, want 1", len(rep.Gaps))
	}

	// The gap should span the two recognized lines around the gap, using
	// the raw line numbers from the file, not positions in a timestamp-only
	// count - unrecognized lines still occupy a line number.
	g := rep.Gaps[0]
	if g.FromLine != 1 || g.ToLine != 3 {
		t.Errorf("gap lines = %d -> %d, want 1 -> 3", g.FromLine, g.ToLine)
	}
}

func TestScanCustomFormat(t *testing.T) {
	f, err := os.Open("testdata/custom_format.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	rep, err := scan(f, "testdata/custom_format.log", 30*time.Second, "2006.01.02-15:04:05")
	if err != nil {
		t.Fatal(err)
	}

	if rep.LinesWithTimestamp != 3 {
		t.Fatalf("LinesWithTimestamp = %d, want 3", rep.LinesWithTimestamp)
	}
	if len(rep.Gaps) != 1 {
		t.Fatalf("len(Gaps) = %d, want 1", len(rep.Gaps))
	}

	g := rep.Gaps[0]
	if g.FromLine != 2 || g.ToLine != 3 {
		t.Errorf("gap lines = %d -> %d, want 2 -> 3", g.FromLine, g.ToLine)
	}
	if g.Seconds != 899 {
		t.Errorf("gap seconds = %v, want 899", g.Seconds)
	}
}

func TestScanMixedBelowMinGapIsIgnored(t *testing.T) {
	f, err := os.Open("testdata/mixed.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	rep, err := scan(f, "testdata/mixed.log", 10*time.Minute, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(rep.Gaps) != 0 {
		t.Errorf("len(Gaps) = %d, want 0 with a 10m threshold", len(rep.Gaps))
	}
}
