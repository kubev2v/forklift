package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var selectRegexp = regexp.MustCompile(`(?i)^(?:(sum|len|any|all)\s*\(?\s*([^)\s]+)\s*\)?|(.+?))\s*(?:as\s+(.+))?$`)

// parseSelectClause splits and parses a select clause into SelectOptions entries.
func parseSelectClause(selectClause string) []SelectOption {
	var opts []SelectOption
	for _, raw := range strings.Split(selectClause, ",") {
		field := strings.TrimSpace(raw)
		if field == "" {
			continue
		}
		if m := selectRegexp.FindStringSubmatch(field); m != nil {
			reducer := strings.ToLower(m[1])
			expr := m[2]
			if expr == "" {
				expr = m[3]
			}
			alias := m[4]
			if alias == "" {
				alias = expr
			}
			if !strings.HasPrefix(expr, ".") && !strings.HasPrefix(expr, "{") {
				expr = "." + expr
			}
			opts = append(opts, SelectOption{
				Field:   expr,
				Alias:   alias,
				Reducer: reducer,
			})
		}
	}
	return opts
}

// parseOrderByClause splits an ORDER BY clause into OrderOption entries.
func parseOrderByClause(orderByClause string, selectOpts []SelectOption) []OrderOption {
	var orderOpts []OrderOption

	for _, rawField := range strings.Split(orderByClause, ",") {
		fieldStr := strings.TrimSpace(rawField)
		if fieldStr == "" {
			continue
		}

		// determine direction
		parts := strings.Fields(fieldStr)
		descending := false
		last := parts[len(parts)-1]
		if strings.EqualFold(last, "desc") {
			descending = true
			parts = parts[:len(parts)-1]
		} else if strings.EqualFold(last, "asc") {
			parts = parts[:len(parts)-1]
		}

		// ensure JSONPath format
		name := strings.Join(parts, " ")
		if !strings.HasPrefix(name, ".") && !strings.HasPrefix(name, "{") {
			name = "." + name
		}

		// find matching select option or create default
		var selOpt SelectOption
		found := false
		for _, sel := range selectOpts {
			if sel.Field == name || sel.Alias == strings.TrimPrefix(name, ".") {
				selOpt = sel
				found = true
				break
			}
		}
		if !found {
			selOpt = SelectOption{
				Field:   name,
				Alias:   strings.TrimPrefix(name, "."),
				Reducer: "",
			}
		}

		orderOpts = append(orderOpts, OrderOption{
			Field:      selOpt,
			Descending: descending,
		})
	}

	return orderOpts
}

// ParseQueryString parses a query string into its component parts
func ParseQueryString(query string) (*QueryOptions, error) {
	options := &QueryOptions{
		Limit: -1, // Default to no limit
	}

	if query == "" {
		return options, nil
	}

	// Validate query syntax before parsing
	if err := ValidateQuerySyntax(query); err != nil {
		return nil, err
	}

	// Convert query to lowercase for case-insensitive matching but preserve original for extraction
	queryLower := strings.ToLower(query)

	// Check for SELECT clause
	selectIndex := strings.Index(queryLower, "select ")
	whereIndex := strings.Index(queryLower, "where ")
	limitIndex := strings.Index(queryLower, "limit ")

	// Look for ordering clause - first "order by", then "sort by" if not found
	orderByIndex := strings.Index(queryLower, "order by ")
	if orderByIndex == -1 {
		orderByIndex = strings.Index(queryLower, "sort by ")
	}

	// Extract SELECT clause if it exists
	if selectIndex >= 0 {
		selectEnd := len(query)
		if whereIndex > selectIndex {
			selectEnd = whereIndex
		} else if orderByIndex > selectIndex {
			selectEnd = orderByIndex
		} else if limitIndex > selectIndex {
			selectEnd = limitIndex
		}

		// Extract select clause (skip "select " prefix which is 7 chars)
		selectClause := strings.TrimSpace(query[selectIndex+7 : selectEnd])
		options.Select = parseSelectClause(selectClause)
		options.HasSelect = len(options.Select) > 0
	}

	// Extract WHERE clause if it exists
	if whereIndex >= 0 {
		whereEnd := len(query)
		if orderByIndex > whereIndex {
			whereEnd = orderByIndex
		} else if limitIndex > whereIndex {
			whereEnd = limitIndex
		}

		// Extract where clause (skip "where " prefix which is 6 chars)
		options.Where = strings.TrimSpace(query[whereIndex+6 : whereEnd])
	}

	// Extract ORDER BY clause if it exists
	if orderByIndex >= 0 {
		orderByEnd := len(query)
		if limitIndex > orderByIndex {
			orderByEnd = limitIndex
		}

		// Extract order by clause (skip 8 chars for both "order by" and "sort by ")
		orderByClause := strings.TrimSpace(query[orderByIndex+8 : orderByEnd])

		// use helper to build OrderOption slice
		options.OrderBy = parseOrderByClause(orderByClause, options.Select)
		options.HasOrderBy = len(options.OrderBy) > 0
	}

	// Extract LIMIT clause using regex (for simplicity with number extraction)
	limitRegex := regexp.MustCompile(`(?i)limit\s+(\d+)`)
	limitMatches := limitRegex.FindStringSubmatch(query)
	if len(limitMatches) > 1 {
		limit, err := strconv.Atoi(limitMatches[1])
		if err != nil {
			return nil, fmt.Errorf("invalid limit value: %v", err)
		}
		options.Limit = limit
		options.HasLimit = true
	}

	return options, nil
}
