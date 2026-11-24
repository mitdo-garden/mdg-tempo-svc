package schedule

import (
	"math"
	"time"
)

const (
	minEase = 1.3
	maxEase = 3.0
	// MaxIntervalDays caps SM-2 growth so Duration math cannot overflow
	// (~106752d = max int64 ns) after many high ratings.
	MaxIntervalDays = 365
)

// Result is the SM-2-lite schedule update (matches mitdo-garden-ui schedule()).
type Result struct {
	Ease         float64
	IntervalDays float64
	Due          time.Time
	Confidence   int
}

// Apply grades a tempo with rating 1–5 and returns the next schedule.
func Apply(ease, intervalDays float64, rating int, now time.Time) Result {
	if rating < 1 {
		rating = 1
	}
	if rating > 5 {
		rating = 5
	}
	if intervalDays < 0 || math.IsNaN(intervalDays) || math.IsInf(intervalDays, 0) {
		intervalDays = 0
	}
	if intervalDays > MaxIntervalDays {
		intervalDays = MaxIntervalDays
	}

	nextEase := ease + float64(rating-3)*0.1
	if nextEase < minEase {
		nextEase = minEase
	}
	if nextEase > maxEase {
		nextEase = maxEase
	}

	now = now.UTC()
	var due time.Time
	var intervalOut float64

	switch rating {
	case 1:
		due = now.Add(10 * time.Minute)
		intervalOut = float64(10*time.Minute) / float64(24*time.Hour)
	case 2:
		due = now.Add(24 * time.Hour)
		intervalOut = 1
	default:
		var days float64
		switch rating {
		case 3:
			days = math.Max(2, math.Round(intervalDays*1.5))
		case 4:
			days = math.Max(4, math.Round(intervalDays*nextEase))
		default: // 5
			days = math.Max(7, math.Round(intervalDays*nextEase*1.4))
		}
		if days > MaxIntervalDays {
			days = MaxIntervalDays
		}
		intervalOut = days
		// AddDate avoids Duration overflow for large day counts.
		due = now.AddDate(0, 0, int(days))
	}

	return Result{
		Ease:         nextEase,
		IntervalDays: intervalOut,
		Due:          due,
		Confidence:   rating,
	}
}
