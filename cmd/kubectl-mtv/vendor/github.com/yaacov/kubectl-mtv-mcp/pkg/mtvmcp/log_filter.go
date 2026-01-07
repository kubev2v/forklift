package mtvmcp

import (
	"regexp"
	"strings"
)

// LogFilter represents log filtering options
type LogFilter struct {
	GrepPattern    string
	GrepInvert     bool
	GrepIgnoreCase bool
	GrepContext    int
	HeadLines      int
	TailLines      int
	MaxLines       int
	MaxBytes       int
}

// LogMetadata contains information about log filtering results
type LogMetadata struct {
	TotalLines       int      `json:"total_lines"`
	ReturnedLines    int      `json:"returned_lines"`
	GrepMatches      int      `json:"grep_matches,omitempty"`
	Truncated        bool     `json:"truncated"`
	TruncationReason string   `json:"truncation_reason,omitempty"`
	FiltersApplied   []string `json:"filters_applied,omitempty"`
}

// FilterLogs applies all filters to log content and returns filtered logs with metadata
func FilterLogs(logs string, filter LogFilter) (string, LogMetadata) {
	if logs == "" {
		return "", LogMetadata{}
	}

	lines := strings.Split(logs, "\n")
	metadata := LogMetadata{
		TotalLines:     len(lines),
		FiltersApplied: []string{},
	}

	// Apply head or tail first (before grep for efficiency)
	if filter.HeadLines > 0 {
		lines = headLines(lines, filter.HeadLines)
		metadata.FiltersApplied = append(metadata.FiltersApplied, "head")
	} else if filter.TailLines > 0 {
		lines = tailLines(lines, filter.TailLines)
		metadata.FiltersApplied = append(metadata.FiltersApplied, "tail")
	}

	// Apply grep filtering with context
	if filter.GrepPattern != "" {
		var matches int
		lines, matches = grepLines(lines, filter.GrepPattern, filter.GrepInvert, filter.GrepIgnoreCase, filter.GrepContext)
		metadata.GrepMatches = matches
		metadata.FiltersApplied = append(metadata.FiltersApplied, "grep")
		if filter.GrepContext > 0 {
			metadata.FiltersApplied = append(metadata.FiltersApplied, "context")
		}
	}

	// Apply output limits
	result := strings.Join(lines, "\n")

	// Check byte limit
	if filter.MaxBytes > 0 && len(result) > filter.MaxBytes {
		result = result[:filter.MaxBytes]
		metadata.Truncated = true
		metadata.TruncationReason = "max_bytes_exceeded"
		metadata.FiltersApplied = append(metadata.FiltersApplied, "byte_limit")
		lines = strings.Split(result, "\n")
	}

	// Check line limit
	if filter.MaxLines > 0 && len(lines) > filter.MaxLines {
		lines = lines[:filter.MaxLines]
		result = strings.Join(lines, "\n")
		metadata.Truncated = true
		metadata.TruncationReason = "max_lines_exceeded"
		metadata.FiltersApplied = append(metadata.FiltersApplied, "line_limit")
	}

	metadata.ReturnedLines = len(lines)

	return result, metadata
}

// grepLines filters lines matching a pattern and returns matching lines with context
func grepLines(lines []string, pattern string, invert, ignoreCase bool, context int) ([]string, int) {
	if pattern == "" {
		return lines, 0
	}

	// Compile regex pattern
	if ignoreCase {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		// If regex compilation fails, return original lines
		return lines, 0
	}

	// Find matching line indices
	matchIndices := make(map[int]bool)
	matchCount := 0

	for i, line := range lines {
		matched := re.MatchString(line)
		if invert {
			matched = !matched
		}
		if matched {
			matchCount++
			// Add the matching line
			matchIndices[i] = true

			// Add context lines
			if context > 0 {
				for j := i - context; j < i; j++ {
					if j >= 0 {
						matchIndices[j] = true
					}
				}
				for j := i + 1; j <= i+context; j++ {
					if j < len(lines) {
						matchIndices[j] = true
					}
				}
			}
		}
	}

	// Extract matching lines in order
	result := make([]string, 0, len(matchIndices))
	for i := 0; i < len(lines); i++ {
		if matchIndices[i] {
			result = append(result, lines[i])
		}
	}

	return result, matchCount
}

// headLines returns the first n lines
func headLines(lines []string, n int) []string {
	if n <= 0 || n >= len(lines) {
		return lines
	}
	return lines[:n]
}

// tailLines returns the last n lines
func tailLines(lines []string, n int) []string {
	if n <= 0 || n >= len(lines) {
		return lines
	}
	return lines[len(lines)-n:]
}
