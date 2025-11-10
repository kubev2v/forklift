package query

import (
	"fmt"

	"github.com/yaacov/tree-search-language/v6/pkg/tsl"
	"github.com/yaacov/tree-search-language/v6/pkg/walkers/semantics"
)

// ParseWhereClause parses a WHERE clause string into a TSL tree
func ParseWhereClause(whereClause string) (*tsl.TSLNode, error) {
	tree, err := tsl.ParseTSL(whereClause)
	if err != nil {
		return nil, fmt.Errorf("failed to parse where clause: %v", err)
	}

	return tree, nil
}

// ApplyFilter filters items using a TSL tree
func ApplyFilter(items []map[string]interface{}, tree *tsl.TSLNode, selectOpts []SelectOption) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	// Filter the items collection using the TSL tree
	for _, item := range items {
		eval := evalFactory(item, selectOpts)

		matchingFilter, err := semantics.Walk(tree, eval)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate where clause: %v", err)
		}

		// Convert interface{} to bool
		if match, ok := matchingFilter.(bool); ok && match {
			results = append(results, item)
		}
	}

	return results, nil
}

// evalFactory gets an item and returns a method that will get the field and return its value
func evalFactory(item map[string]interface{}, selectOpts []SelectOption) semantics.EvalFunc {
	return func(k string) (interface{}, bool) {
		// Use GetValue to respect aliases and reducers
		if v, err := GetValue(item, k, selectOpts); err == nil {
			return v, true
		}
		return nil, true
	}
}
