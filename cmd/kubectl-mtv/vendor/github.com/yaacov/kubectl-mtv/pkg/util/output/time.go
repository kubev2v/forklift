package output

import (
	"time"
)

// FormatTime formats a timestamp string with optional UTC conversion
func FormatTime(timestamp string, useUTC bool) string {
	if timestamp == "" {
		return "N/A"
	}

	// Parse the timestamp
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return timestamp
	}

	// Convert to UTC or local time as requested
	if useUTC {
		t = t.UTC()
	} else {
		t = t.Local()
	}

	// Format as "2006-01-02 15:04:05"
	return t.Format("2006-01-02 15:04:05")
}

// FormatTimestamp formats a time.Time object with optional UTC conversion
func FormatTimestamp(timestamp time.Time, useUTC bool) string {
	// Convert to UTC or local time as requested
	if useUTC {
		timestamp = timestamp.UTC()
	} else {
		timestamp = timestamp.Local()
	}

	// Format as "2006-01-02 15:04:05"
	return timestamp.Format("2006-01-02 15:04:05")
}
