package timeutil

import "time"

func StartOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func SameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func MinutesFromMidnight(value time.Time) int {
	return value.Hour()*60 + value.Minute()
}
