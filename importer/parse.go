package importer

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func parseGermanDecimalHoursToMinutes(raw string) (int, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return 0, nil
	}
	if strings.Contains(cleaned, ",") {
		if strings.Contains(cleaned, ".") {
			cleaned = strings.ReplaceAll(cleaned, ".", "")
		}
		cleaned = strings.ReplaceAll(cleaned, ",", ".")
	}

	hours, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("parse hours %q: %w", raw, err)
	}

	minutes := int(math.Round(hours * 60))
	if minutes < 0 {
		return 0, fmt.Errorf("hours must not be negative")
	}
	return minutes, nil
}

func parseMinutes(raw string) (int, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return 0, nil
	}

	if strings.Contains(cleaned, ",") {
		cleaned = strings.ReplaceAll(cleaned, ",", ".")
	}

	minutes, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("parse minutes %q: %w", raw, err)
	}

	rounded := int(math.Round(minutes))
	if rounded < 0 {
		return 0, fmt.Errorf("minutes must not be negative")
	}
	return rounded, nil
}

func parseDateAndTime(dateValue, timeValue string) (time.Time, error) {
	dateValue = strings.TrimSpace(dateValue)
	timeValue = strings.TrimSpace(timeValue)
	if dateValue == "" || timeValue == "" {
		return time.Time{}, fmt.Errorf("missing date or time")
	}

	datetime := dateValue + " " + timeValue
	layouts := []string{
		"02.01.2006 03:04 PM",
		"02.01.2006 15:04",
		"2006-01-02 15:04",
		"2006-01-02 03:04 PM",
	}

	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, datetime, time.Local); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported date/time format: %q", datetime)
}

func parseDateTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty datetime")
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04",
		"02.01.2006 15:04",
		"02.01.2006 03:04 PM",
	}

	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported datetime format: %q", value)
}
