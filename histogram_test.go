package main

import (
	"testing"
	"time"
)

func gapOfSeconds(seconds float64) gap {
	return gap{Seconds: seconds}
}

func TestBuildHistogramNoGaps(t *testing.T) {
	buckets := buildHistogram(nil, 30*time.Second)
	if len(buckets) != 0 {
		t.Errorf("len(buckets) = %d, want 0", len(buckets))
	}
}

func TestBuildHistogramSingleGap(t *testing.T) {
	gaps := []gap{gapOfSeconds(45)}
	buckets := buildHistogram(gaps, 30*time.Second)

	if len(buckets) != 1 {
		t.Fatalf("len(buckets) = %d, want 1", len(buckets))
	}
	if buckets[0].MinSeconds != 30 {
		t.Errorf("MinSeconds = %v, want 30", buckets[0].MinSeconds)
	}
	if buckets[0].MaxSeconds != nil {
		t.Errorf("MaxSeconds = %v, want nil (unbounded)", *buckets[0].MaxSeconds)
	}
	if buckets[0].Count != 1 {
		t.Errorf("Count = %d, want 1", buckets[0].Count)
	}
}

func TestBuildHistogramMultipleBuckets(t *testing.T) {
	// lower = 10, so bucket starts double: 10, 20, 40, 80, 160, 320.
	// 160-320 gets no gap and should be dropped from the result.
	gaps := []gap{
		gapOfSeconds(12),
		gapOfSeconds(25),
		gapOfSeconds(50),
		gapOfSeconds(100),
		gapOfSeconds(500),
	}
	buckets := buildHistogram(gaps, 10*time.Second)

	wantMins := []float64{10, 20, 40, 80, 320}
	if len(buckets) != len(wantMins) {
		t.Fatalf("len(buckets) = %d, want %d: %+v", len(buckets), len(wantMins), buckets)
	}
	for i, want := range wantMins {
		if buckets[i].MinSeconds != want {
			t.Errorf("buckets[%d].MinSeconds = %v, want %v", i, buckets[i].MinSeconds, want)
		}
		if buckets[i].Count != 1 {
			t.Errorf("buckets[%d].Count = %d, want 1", i, buckets[i].Count)
		}
	}
	if buckets[len(buckets)-1].MaxSeconds != nil {
		t.Errorf("last bucket MaxSeconds = %v, want nil (unbounded)", *buckets[len(buckets)-1].MaxSeconds)
	}
	for i := 0; i < len(buckets)-1; i++ {
		if buckets[i].MaxSeconds == nil {
			t.Errorf("buckets[%d].MaxSeconds = nil, want a bound", i)
			continue
		}
		if *buckets[i].MaxSeconds != wantMins[i]*2 {
			t.Errorf("buckets[%d].MaxSeconds = %v, want %v", i, *buckets[i].MaxSeconds, wantMins[i]*2)
		}
	}
}

func TestBuildHistogramGapsInSameBucketAreCounted(t *testing.T) {
	gaps := []gap{gapOfSeconds(31), gapOfSeconds(35), gapOfSeconds(59)}
	buckets := buildHistogram(gaps, 30*time.Second)

	if len(buckets) != 1 {
		t.Fatalf("len(buckets) = %d, want 1: %+v", len(buckets), buckets)
	}
	if buckets[0].Count != 3 {
		t.Errorf("Count = %d, want 3", buckets[0].Count)
	}
}

func TestBuildHistogramZeroMinGapDoesNotHang(t *testing.T) {
	gaps := []gap{gapOfSeconds(5)}
	buckets := buildHistogram(gaps, 0)

	if len(buckets) != 1 {
		t.Fatalf("len(buckets) = %d, want 1", len(buckets))
	}
	if buckets[0].MinSeconds != 1 {
		t.Errorf("MinSeconds = %v, want 1", buckets[0].MinSeconds)
	}
}
