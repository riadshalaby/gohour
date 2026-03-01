package timeutil

import (
	"testing"
	"time"
)

func TestStartOfDay(t *testing.T) {
	t.Parallel()

	input := time.Date(2026, 3, 1, 14, 37, 9, 123, time.Local)
	got := StartOfDay(input)

	if got.Year() != 2026 || got.Month() != time.March || got.Day() != 1 {
		t.Fatalf("unexpected date: %v", got)
	}
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 || got.Nanosecond() != 0 {
		t.Fatalf("expected midnight, got %v", got)
	}
}

func TestSameDay(t *testing.T) {
	t.Parallel()

	a := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	b := time.Date(2026, 3, 1, 18, 30, 0, 0, time.Local)
	c := time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)

	if !SameDay(a, b) {
		t.Fatalf("expected same day for %v and %v", a, b)
	}
	if SameDay(a, c) {
		t.Fatalf("expected different days for %v and %v", a, c)
	}
}

func TestMinutesFromMidnight(t *testing.T) {
	t.Parallel()

	input := time.Date(2026, 3, 1, 13, 25, 0, 0, time.Local)
	if got := MinutesFromMidnight(input); got != 805 {
		t.Fatalf("expected 805, got %d", got)
	}
}
