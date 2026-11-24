package schedule

import (
	"testing"
	"time"
)

func TestApplyRating1(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	got := Apply(2.3, 3, 1, now)
	if got.Confidence != 1 {
		t.Fatalf("confidence=%d", got.Confidence)
	}
	if got.Due.Sub(now) != 10*time.Minute {
		t.Fatalf("due delta=%v", got.Due.Sub(now))
	}
}

func TestApplyRating5(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	got := Apply(2.3, 5, 5, now)
	if got.Confidence != 5 {
		t.Fatalf("confidence=%d", got.Confidence)
	}
	if got.Ease < 2.4 || got.Ease > 2.6 {
		t.Fatalf("ease=%v", got.Ease)
	}
	if got.IntervalDays < 7 {
		t.Fatalf("interval=%v", got.IntervalDays)
	}
}

func TestApplyCapsHugeInterval(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	got := Apply(3.0, 1e9, 5, now)
	if got.IntervalDays != MaxIntervalDays {
		t.Fatalf("interval=%v want %v", got.IntervalDays, MaxIntervalDays)
	}
	want := now.AddDate(0, 0, MaxIntervalDays)
	if !got.Due.Equal(want) {
		t.Fatalf("due=%v want %v", got.Due, want)
	}
}
