package query

import (
	"fmt"
)

// applyReducer runs sum/len/any/all on slice values.
func applyReducer(value interface{}, reducer string) (interface{}, error) {
	arr, ok := value.([]interface{})
	if !ok {
		// not an array – no reduction
		return value, nil
	}

	switch reducer {
	case "sum":
		var total float64
		for _, v := range arr {
			switch n := v.(type) {
			case float64:
				total += n
			case int:
				total += float64(n)
			case int64:
				total += float64(n)
			default:
				// skip non‐numeric
			}
		}
		return total, nil

	case "len":
		return len(arr), nil

	case "any":
		for _, v := range arr {
			if b, ok := v.(bool); ok && b {
				return true, nil
			}
		}
		return false, nil

	case "all":
		if len(arr) == 0 {
			return false, nil
		}
		for _, v := range arr {
			if b, ok := v.(bool); !ok || !b {
				return false, nil
			}
		}
		return true, nil

	default:
		return value, fmt.Errorf("unknown reducer %q", reducer)
	}
}
