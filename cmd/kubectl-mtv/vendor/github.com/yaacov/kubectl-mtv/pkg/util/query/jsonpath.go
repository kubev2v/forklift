package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// GetValueByPathString gets a value from an object using a string path
// The path can use dot notation (e.g. "metadata.name") and array indexing (e.g. "spec.containers[0].name")
func GetValueByPathString(obj interface{}, path string) (interface{}, error) {
	// Remove JSONPath notation if present
	path = strings.TrimPrefix(path, "{{")
	path = strings.TrimSuffix(path, "}}")
	path = strings.TrimSpace(path)

	// Remove leading dot if present
	path = strings.TrimPrefix(path, ".")

	// Split the path into parts, handling brackets and dots
	// e.g. "spec.containers[0].name" -> ["spec", "containers[0]", "name"]
	var parts []string
	var currentPart strings.Builder
	insideBrackets := false

	for _, char := range path {
		switch char {
		case '[':
			insideBrackets = true
			currentPart.WriteRune(char)
		case ']':
			insideBrackets = false
			currentPart.WriteRune(char)
		case '.':
			if insideBrackets {
				// If inside brackets, the dot is part of the current segment
				currentPart.WriteRune(char)
			} else {
				// If outside brackets, the dot is a separator
				parts = append(parts, currentPart.String())
				currentPart.Reset()
			}
		default:
			currentPart.WriteRune(char)
		}
	}

	// Add the last part if there's anything left
	if currentPart.Len() > 0 {
		parts = append(parts, currentPart.String())
	}

	return getValueByPath(obj, parts)
}

// getValueByPath recursively traverses an object following a path
func getValueByPath(obj interface{}, pathParts []string) (interface{}, error) {
	if len(pathParts) == 0 {
		return obj, nil
	}

	if obj == nil {
		return nil, fmt.Errorf("cannot access %s on nil value", strings.Join(pathParts, "."))
	}

	part := pathParts[0]
	remainingParts := pathParts[1:]

	// Check if part has array indexing notation [i], map key notation [key], or wildcard [*]
	arrayIndex := -1
	mapKey := ""
	isWildcard := false

	// Run regex matchers
	wildcardMatch := regexp.MustCompile(`(.*)\[\*\]$`).FindStringSubmatch(part)
	arrayMatch := regexp.MustCompile(`(.*)\[(\d+)\]$`).FindStringSubmatch(part)
	mapMatch := regexp.MustCompile(`(.*)\[([^\]]+)\]$`).FindStringSubmatch(part)

	// Then use flat if conditions to process matches
	if len(wildcardMatch) == 2 {
		part = wildcardMatch[1]
		isWildcard = true
	} else if len(arrayMatch) == 3 {
		part = arrayMatch[1]
		index, err := strconv.Atoi(arrayMatch[2])
		if err != nil {
			return nil, fmt.Errorf("invalid array index in path: %s", part)
		}
		arrayIndex = index
	} else if len(mapMatch) == 3 {
		part = mapMatch[1]
		mapKey = mapMatch[2]
		// Remove quotes if present
		mapKey = strings.Trim(mapKey, `"'`)
	}

	switch objTyped := obj.(type) {
	case map[string]interface{}:
		// Get value for current part
		value, exists := objTyped[part]
		if !exists {
			// Don't fail if the part is not found, just return nil
			return nil, nil
		}

		// Handle wildcard for arrays
		if isWildcard {
			// Check if the value is an array
			arr, ok := value.([]interface{})
			if !ok {
				return nil, fmt.Errorf("cannot apply wildcard to non-array value: %s", part)
			}

			// For wildcard, collect results from all array elements
			var results []interface{}
			for _, item := range arr {
				result, err := getValueByPath(item, remainingParts)
				if err == nil && result != nil {
					// Check if result is an array itself and flatten if needed
					if resultArray, isArray := result.([]interface{}); isArray {
						// Flatten by appending individual elements
						results = append(results, resultArray...)
					} else {
						// Non-array result, append as is
						results = append(results, result)
					}
				}
			}
			return results, nil
		}

		// Handle array indexing if present
		if arrayIndex >= 0 {
			// Check if the value is an array
			if arr, ok := value.([]interface{}); ok {
				if arrayIndex >= len(arr) {
					return nil, fmt.Errorf("array index out of bounds: %d", arrayIndex)
				}
				value = arr[arrayIndex]
			} else {
				return nil, fmt.Errorf("cannot apply array index to non-array value: %s", part)
			}
		}

		// Handle map key access if present
		if mapKey != "" {
			// Check if the value is a map
			if m, ok := value.(map[string]interface{}); ok {
				mapValue, exists := m[mapKey]
				if !exists {
					return nil, nil
				}
				value = mapValue
			} else {
				return nil, fmt.Errorf("cannot apply map key to non-map value: %s", part)
			}
		}

		// If this is the last part, return the value
		if len(remainingParts) == 0 {
			return value, nil
		}

		// Otherwise, continue recursing
		return getValueByPath(value, remainingParts)

	default:
		return nil, fmt.Errorf("cannot access property %s on non-object value", part)
	}
}

// GetValue retrieves a value by name (or alias) from obj using JSONPath,
// then applies a reducer if one is specified in selectOpts.
func GetValue(obj interface{}, name string, selectOpts []SelectOption) (interface{}, error) {
	path := name
	var reducer string

	// If name matches an alias, switch to its Field and capture reducer
	for _, opt := range selectOpts {
		if opt.Alias == name {
			path = opt.Field
			reducer = opt.Reducer
			break
		}
	}

	// Fetch the raw value
	val, err := GetValueByPathString(obj, path)
	if err != nil {
		return nil, err
	}

	// Apply reducer if specified
	if reducer != "" {
		reduced, err := applyReducer(val, reducer)
		if err != nil {
			return nil, fmt.Errorf("failed to apply reducer %q: %v", reducer, err)
		}
		return reduced, nil
	}

	return val, nil
}
