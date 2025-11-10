package query

// ApplyQueryInterface parses the query string and applies it to the data (interface{}).
// Accepts data as []map[string]interface{} or map[string]interface{}.
// Returns filtered data as interface{} and error.
func ApplyQueryInterface(data interface{}, query string) (interface{}, error) {
	var items []map[string]interface{}

	switch v := data.(type) {
	case []map[string]interface{}:
		items = v
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
