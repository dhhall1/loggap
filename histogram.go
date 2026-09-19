package main

import "time"

// histBucket is one bucket of a gap size histogram: every gap with
// MinSeconds <= duration < MaxSeconds falls in it. A nil MaxSeconds marks
// the last bucket, which is unbounded above.
type histBucket struct {
	MinSeconds float64  `json:"min_seconds"`
	MaxSeconds *float64 `json:"max_seconds,omitempty"`
	Count      int      `json:"count"`
}

// buildHistogram buckets gap durations into powers-of-two multiples of
// minGap, starting at the threshold itself and doubling up past the
// largest gap found. Doubling keeps the bucket count small and readable
// whether the gaps in a log span seconds or days, without needing a fixed
// bucket width picked per log. Buckets with no gaps in them are dropped
// rather than printed as a run of zeros.
func buildHistogram(gaps []gap, minGap time.Duration) []histBucket {
	if len(gaps) == 0 {
		return []histBucket{}
	}

	lower := minGap.Seconds()
	if lower <= 0 {
		lower = 1
	}

	largest := gaps[0].Seconds
	for _, g := range gaps[1:] {
		if g.Seconds > largest {
			largest = g.Seconds
		}
	}

	var starts []float64
	for edge := lower; edge <= largest; edge *= 2 {
		starts = append(starts, edge)
	}

	buckets := make([]histBucket, len(starts))
	for i, s := range starts {
		buckets[i].MinSeconds = s
		if i+1 < len(starts) {
			max := starts[i+1]
			buckets[i].MaxSeconds = &max
		}
	}

	for _, g := range gaps {
		for i := range buckets {
			if buckets[i].MaxSeconds == nil || g.Seconds < *buckets[i].MaxSeconds {
				buckets[i].Count++
				break
			}
		}
	}

	nonEmpty := buckets[:0]
	for _, b := range buckets {
		if b.Count > 0 {
			nonEmpty = append(nonEmpty, b)
		}
	}
	return nonEmpty
}
