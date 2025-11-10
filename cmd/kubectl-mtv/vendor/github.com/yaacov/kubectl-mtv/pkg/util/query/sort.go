package query

import (
	"fmt"
	"sort"
	"strconv"
)

// SortItems sorts the items based on the provided ordering options
func SortItems(items []map[string]interface{}, queryOpts *QueryOptions) ([]map[string]interface{}, error) {
	orderOpts := queryOpts.OrderBy
	selectOpts := queryOpts.Select

	if len(orderOpts) == 0 {
		return items, nil
	}

	// Create a copy of the items to avoid modifying the original
	result := make([]map[string]interface{}, len(items))
	copy(result, items)

	// Sort the items
	sort.SliceStable(result, func(i, j int) bool {
		for _, orderOpt := range orderOpts {
			// Use GetValue to retrieve values respecting aliases and reducers
			name := orderOpt.Field.Alias
			valueI, err := GetValue(result[i], name, selectOpts)
			if err != nil {
				continue
			}
			valueJ, err := GetValue(result[j], name, selectOpts)
			if err != nil {
				continue
			}

			// Try to convert string values to numeric types if possible
			valueI = convertStringToNumeric(valueI)
			valueJ = convertStringToNumeric(valueJ)

			// Compare values
			cmp := compareValues(valueI, valueJ)
			if cmp == 0 {
				// equal on this field, try next
				continue
			}

			// If descending, reverse the comparison
			if orderOpt.Descending {
				return cmp > 0
			}
			return cmp < 0
		}

		// all equal
		return false
	})

	return result, nil
}

// convertStringToNumeric attempts to convert string values to numeric types
func convertStringToNumeric(value interface{}) interface{} {
	if strValue, ok := value.(string); ok {
		// Try to convert to numeric types if possible
		if i, err := strconv.ParseInt(strValue, 10, 64); err == nil {
			return int(i)
		}

		if f, err := strconv.ParseFloat(strValue, 64); err == nil {
			return f
		}
	}
	return value
}

// compareValues compares two values for sorting
func compareValues(a, b interface{}) int {
	// Handle nil values
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	// Convert to comparable types
	switch aVal := a.(type) {
	case string:
		if bVal, ok := b.(string); ok {
			if aVal < bVal {
				return -1
			}
			if aVal > bVal {
				return 1
			}
			return 0
		}
	case int:
		if bVal, ok := b.(int); ok {
			return aVal - bVal
		}
	case float64:
		if bVal, ok := b.(float64); ok {
			if aVal < bVal {
				return -1
			}
			if aVal > bVal {
				return 1
			}
			return 0
		}
	}

	// Default string comparison
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	if aStr < bStr {
		return -1
	}
	if aStr > bStr {
		return 1
	}
	return 0
}
