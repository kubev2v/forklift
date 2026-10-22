package query

import "fmt"

// ApplyQueryInterface parses the query string and applies it to the data (interface{}).
// Accepts data as []map[string]interface{}, []interface{} (with map elements), or map[string]interface{}.
// Returns an error if any element in a []interface{} slice is not a map[string]interface{}.
func ApplyQueryInterface(data interface{}, query string) (interface{}, error) {
	var items []map[string]interface{}

	switch v := data.(type) {
	case []map[string]interface{}:
		items = v
	case []interface{}:
		items = make([]map[string]interface{}, 0, len(v))
		for i, elem := range v {
			m, ok := elem.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("element at index %d has unsupported type %T, expected map[string]interface{}", i, elem)
			}
			items = append(items, m)
		}
	case map[string]interface{}:
		items = []map[string]interface{}{v}
	default:
		return data, nil // If not a supported type, return as-is
	}

	queryOpts, err := ParseQueryString(query)
	if err != nil {
		return nil, err
	}

	filtered, err := ApplyQuery(items, queryOpts)
	if err != nil {
		return nil, err
	}
	return filtered, nil
}
