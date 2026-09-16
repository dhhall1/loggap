package main

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanServiceLog(t *testing.T) {
	f, err := os.Open("testdata/service.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	rep, err := scan(f, "testdata/service.log", 30*time.Second, "", nil, nil)
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

func TestScanSinceExcludesEarlierLines(t *testing.T) {
	f, err := os.Open("testdata/service.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// service.log's gap runs from line 3 (09:00:02) to line 4 (09:14:57).
	// A --since that lands after line 3 but before line 4 should drop the
	// earlier side of the gap along with everything before it.
	since, err := time.Parse(time.RFC3339, "2024-03-01T09:10:00Z")
	if err != nil {
		t.Fatal(err)
	}

	rep, err := scan(f, "testdata/service.log", 30*time.Second, "", &since, nil)
	if err != nil {
		t.Fatal(err)
	}

	if rep.LinesWithTimestamp != 2 {
		t.Errorf("LinesWithTimestamp = %d, want 2", rep.LinesWithTimestamp)
	}
	if len(rep.Gaps) != 0 {
		t.Errorf("len(Gaps) = %d, want 0 once the line before the gap is excluded", len(rep.Gaps))
	}
}

func TestScanUntilExcludesLaterLines(t *testing.T) {
	f, err := os.Open("testdata/service.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	until, err := time.Parse(time.RFC3339, "2024-03-01T09:00:02Z")
	if err != nil {
		t.Fatal(err)
	}

	rep, err := scan(f, "testdata/service.log", 30*time.Second, "", nil, &until)
	if err != nil {
		t.Fatal(err)
	}

	if rep.LinesWithTimestamp != 3 {
		t.Errorf("LinesWithTimestamp = %d, want 3", rep.LinesWithTimestamp)
	}
	if len(rep.Gaps) != 0 {
		t.Errorf("len(Gaps) = %d, want 0 once the line after the gap is excluded", len(rep.Gaps))
	}
}

func TestScanNoGaps(t *testing.T) {
	f, err := os.Open("testdata/no_gaps.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	rep, err := scan(f, "testdata/no_gaps.log", 30*time.Second, "", nil, nil)
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

	rep, err := scan(f, "testdata/mixed.log", 30*time.Second, "", nil, nil)
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

	rep, err := scan(f, "testdata/custom_format.log", 30*time.Second, "2006.01.02-15:04:05", nil, nil)
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

func TestOpenInputGzip(t *testing.T) {
	raw, err := os.ReadFile("testdata/service.log")
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "service.log.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := openInput(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	rep, err := scan(r, path, 30*time.Second, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if rep.LinesScanned != 5 {
		t.Errorf("LinesScanned = %d, want 5", rep.LinesScanned)
	}
	if len(rep.Gaps) != 1 {
		t.Fatalf("len(Gaps) = %d, want 1", len(rep.Gaps))
	}
	if rep.Gaps[0].Seconds != 895 {
		t.Errorf("gap seconds = %v, want 895", rep.Gaps[0].Seconds)
	}
}

func TestOpenInputGzipInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.gz")
	if err := os.WriteFile(path, []byte("not actually gzip data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := openInput(path); err == nil {
		t.Fatal("openInput on a non-gzip .gz file: got nil error, want one")
	}
}

func TestScanMixedBelowMinGapIsIgnored(t *testing.T) {
	f, err := os.Open("testdata/mixed.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	rep, err := scan(f, "testdata/mixed.log", 10*time.Minute, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(rep.Gaps) != 0 {
		t.Errorf("len(Gaps) = %d, want 0 with a 10m threshold", len(rep.Gaps))
	}
}

func TestParseTimeFlag(t *testing.T) {
	want := time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)

	cases := []string{
		"2024-03-01T09:00:00Z",
		"2024-03-01T09:00:00",
		"2024-03-01 09:00:00",
	}
	for _, in := range cases {
		got, err := parseTimeFlag(in)
		if err != nil {
			t.Errorf("parseTimeFlag(%q): %v", in, err)
			continue
		}
		if !got.Equal(want) {
			t.Errorf("parseTimeFlag(%q) = %v, want %v", in, got, want)
		}
	}

	if _, err := parseTimeFlag("not a time"); err == nil {
		t.Error("parseTimeFlag(\"not a time\"): got nil error, want one")
	}
}
