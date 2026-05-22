package inventory

// ExtractEC2Objects processes EC2 inventory data by extracting the "object" field
// and discarding the envelope. This should be called immediately after fetching
// EC2 inventory data from the API server.
//
// The EC2 inventory server returns data in this format:
//
//	{
//	  "id": "...",
//	  "name": "...",
//	  "object": { ... actual EC2 data ... },
//	  "provider": "ec2",
//	  ...
//	}
//
// This function extracts the "object" content and merges it with id and name,
// returning a flattened structure for easier processing.
func ExtractEC2Objects(data interface{}) interface{} {
	switch v := data.(type) {
	case []interface{}:
		result := make([]interface{}, 0, len(v))
		for _, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				result = append(result, extractEC2Object(itemMap))
			}
		}
		return result
	case map[string]interface{}:
		return extractEC2Object(v)
	default:
		return data
	}
}

// extractEC2Object extracts the object field from a single EC2 inventory item
func extractEC2Object(item map[string]interface{}) map[string]interface{} {
	// If there's no object field, return the item as-is (backward compatibility)
	obj, hasObject := item["object"].(map[string]interface{})
	if !hasObject {
		return item
	}

	// Add top-level id and name fields from the envelope to the object
	if id, ok := item["id"]; ok {
		obj["id"] = id
	}
	if name, ok := item["name"]; ok {
		obj["name"] = name
	}

	return obj
}
