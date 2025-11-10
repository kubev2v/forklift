package semantics

import "time"

func toDate(value interface{}) (time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		return v, true
	case string:
		// try full RFC3339 first
		if date, err := time.Parse(time.RFC3339, v); err == nil {
			return date, true
		}
		// then try short date format YYYY-MM-DD
		if date, err := time.Parse("2006-01-02", v); err == nil {
			return date, true
		}
	}
	return time.Time{}, false
}
