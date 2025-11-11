package query

import (
	"fmt"
	"regexp"
	"strings"
)

// QueryKeyword represents a SQL-like keyword in queries
type QueryKeyword struct {
	Name     string
	Pattern  *regexp.Regexp
	Position int // Expected position order (1=SELECT, 2=WHERE, 3=ORDER BY, 4=LIMIT)
}

// Common query keywords with their expected positions
var keywords = []QueryKeyword{
	{"SELECT", regexp.MustCompile(`(?i)\bselect\b`), 1},
	{"WHERE", regexp.MustCompile(`(?i)\bwhere\b`), 2},
	{"ORDER BY", regexp.MustCompile(`(?i)\border\s+by\b`), 3},
	{"SORT BY", regexp.MustCompile(`(?i)\bsort\s+by\b`), 3}, // Same position as ORDER BY
	{"LIMIT", regexp.MustCompile(`(?i)\blimit\b`), 4},
}

// Common typos and their corrections
var typoCorrections = map[string]string{
	"selct":  "SELECT",
	"slect":  "SELECT",
	"select": "SELECT", // for case consistency
	"wher":   "WHERE",
	"were":   "WHERE",
	"whre":   "WHERE",
	"limt":   "LIMIT",
	"limit":  "LIMIT", // for case consistency
	"lmit":   "LIMIT",
	"oder":   "ORDER",
	"order":  "ORDER", // for case consistency
	"ordr":   "ORDER",
	"srot":   "SORT",
	"sort":   "SORT", // for case consistency
	"sotr":   "SORT",
}

// ValidationError represents a query syntax error with suggestions
type ValidationError struct {
	Type       string
	Message    string
	Suggestion string
	Position   int // Character position in query (if applicable)
	FoundText  string
}

// Error implements the error interface
func (e ValidationError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("%s: %s. Suggestion: %s", e.Type, e.Message, e.Suggestion)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// KeywordOccurrence tracks where and how many times a keyword appears
type KeywordOccurrence struct {
	Keyword   QueryKeyword
	Count     int
	Positions []int
}

// ValidateQuerySyntax performs comprehensive syntax validation on a query string
func ValidateQuerySyntax(query string) error {
	if query == "" {
		return nil
	}

	// Check for typos first
	if err := checkTypos(query); err != nil {
		return err
	}

	// Count keyword occurrences
	occurrences := countKeywordOccurrences(query)

	// Check for duplicate keywords
	if err := checkDuplicateKeywords(occurrences); err != nil {
		return err
	}

	// Check clause ordering
	if err := checkClauseOrdering(occurrences); err != nil {
		return err
	}

	// Check for conflicting keywords (ORDER BY vs SORT BY)
	if err := checkConflictingKeywords(occurrences); err != nil {
		return err
	}

	// Check for empty clauses
	if err := checkEmptyClauses(query, occurrences); err != nil {
		return err
	}

	return nil
}

// checkTypos looks for common keyword typos and suggests corrections
func checkTypos(query string) error {
	queryLower := strings.ToLower(query)
	words := regexp.MustCompile(`\b\w+\b`).FindAllString(queryLower, -1)

	for _, word := range words {
		if correction, found := typoCorrections[word]; found && word != strings.ToLower(correction) {
			// Make sure this isn't actually a valid keyword
			isValidKeyword := false
			for _, kw := range keywords {
				if kw.Pattern.MatchString(word) {
					isValidKeyword = true
					break
				}
			}

			if !isValidKeyword {
				return ValidationError{
					Type:       "Keyword Typo",
					Message:    fmt.Sprintf("Unrecognized keyword '%s'", word),
					Suggestion: fmt.Sprintf("Did you mean '%s'?", correction),
					FoundText:  word,
				}
			}
		}
	}

	return nil
}

// countKeywordOccurrences finds all occurrences of each keyword in the query
func countKeywordOccurrences(query string) []KeywordOccurrence {
	var occurrences []KeywordOccurrence

	for _, keyword := range keywords {
		matches := keyword.Pattern.FindAllStringIndex(query, -1)
		if len(matches) > 0 {
			positions := make([]int, len(matches))
			for i, match := range matches {
				positions[i] = match[0]
			}
			occurrences = append(occurrences, KeywordOccurrence{
				Keyword:   keyword,
				Count:     len(matches),
				Positions: positions,
			})
		}
	}

	return occurrences
}

// checkDuplicateKeywords validates that keywords don't appear multiple times
func checkDuplicateKeywords(occurrences []KeywordOccurrence) error {
	for _, occ := range occurrences {
		if occ.Count > 1 {
			return ValidationError{
				Type:       "Duplicate Keyword",
				Message:    fmt.Sprintf("Keyword '%s' appears %d times", occ.Keyword.Name, occ.Count),
				Suggestion: fmt.Sprintf("Use '%s' only once in your query", occ.Keyword.Name),
			}
		}
	}
	return nil
}

// checkClauseOrdering validates that clauses appear in the correct SQL-like order
func checkClauseOrdering(occurrences []KeywordOccurrence) error {
	// Create a map of positions to keywords for easier sorting
	positionMap := make(map[int]QueryKeyword)
	for _, occ := range occurrences {
		if len(occ.Positions) > 0 {
			positionMap[occ.Positions[0]] = occ.Keyword
		}
	}

	// Get all positions and sort them
	var positions []int
	for pos := range positionMap {
		positions = append(positions, pos)
	}

	// Simple bubble sort for positions
	for i := 0; i < len(positions); i++ {
		for j := 0; j < len(positions)-1-i; j++ {
			if positions[j] > positions[j+1] {
				positions[j], positions[j+1] = positions[j+1], positions[j]
			}
		}
	}

	// Check if keywords appear in correct order
	lastPosition := 0
	for _, pos := range positions {
		keyword := positionMap[pos]
		if keyword.Position < lastPosition {
			return ValidationError{
				Type:       "Invalid Clause Order",
				Message:    fmt.Sprintf("'%s' appears after a clause that should come later", keyword.Name),
				Suggestion: "Use the order: SELECT → WHERE → ORDER BY/SORT BY → LIMIT",
			}
		}
		lastPosition = keyword.Position
	}

	return nil
}

// checkConflictingKeywords checks for mutually exclusive keywords
func checkConflictingKeywords(occurrences []KeywordOccurrence) error {
	hasOrderBy := false
	hasSortBy := false

	for _, occ := range occurrences {
		switch occ.Keyword.Name {
		case "ORDER BY":
			hasOrderBy = true
		case "SORT BY":
			hasSortBy = true
		}
	}

	if hasOrderBy && hasSortBy {
		return ValidationError{
			Type:       "Conflicting Keywords",
			Message:    "Cannot use both 'ORDER BY' and 'SORT BY' in the same query",
			Suggestion: "Choose either 'ORDER BY' or 'SORT BY', not both",
		}
	}

	return nil
}

// checkEmptyClauses validates that clauses have content after the keyword
func checkEmptyClauses(query string, occurrences []KeywordOccurrence) error {
	queryLower := strings.ToLower(query)

	for _, occ := range occurrences {
		if len(occ.Positions) == 0 {
			continue
		}

		pos := occ.Positions[0]
		keywordName := strings.ToLower(occ.Keyword.Name)

		// Find the end of the keyword
		keywordEndPos := pos + len(keywordName)
		if strings.Contains(keywordName, " ") {
			// For multi-word keywords like "ORDER BY", we need to be more careful
			pattern := strings.ReplaceAll(keywordName, " ", `\s+`)
			regex := regexp.MustCompile(`(?i)` + pattern)
			match := regex.FindStringIndex(queryLower[pos:])
			if match != nil {
				keywordEndPos = pos + match[1]
			}
		}

		// Extract everything after this keyword until the next keyword or end of query
		afterKeyword := query[keywordEndPos:]
		nextKeywordPos := len(afterKeyword)

		// Find the next keyword position
		for _, nextOcc := range occurrences {
			for _, nextPos := range nextOcc.Positions {
				if nextPos > keywordEndPos {
					relativePos := nextPos - keywordEndPos
					if relativePos < nextKeywordPos {
						nextKeywordPos = relativePos
					}
				}
			}
		}

		clauseContent := strings.TrimSpace(afterKeyword[:nextKeywordPos])

		if clauseContent == "" {
			return ValidationError{
				Type:       "Empty Clause",
				Message:    fmt.Sprintf("'%s' keyword found but no content follows", occ.Keyword.Name),
				Suggestion: fmt.Sprintf("Add content after '%s' or remove the keyword", occ.Keyword.Name),
			}
		}
	}

	return nil
}
