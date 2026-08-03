package tui

import (
	"regexp"
	"strings"
)

// ANSI escape codes for search highlighting
const (
	highlightOn  = "\033[7m"      // reverse video
	highlightOff = "\033[27m"     // reverse video off
	focusOn      = "\033[1;33;7m" // bold + yellow + reverse
	focusOff     = "\033[0m"
)

// matchInfo records the position of a search match in the content.
type matchInfo struct {
	Line   int
	Column int // column in the ANSI-stripped text
}

// colMatch pairs a column position with whether this match is the focused one.
type colMatch struct {
	col     int
	focused bool
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripAnsiCodes removes all ANSI escape sequences from s.
func stripAnsiCodes(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

// findMatches locates all case-insensitive occurrences of term in content,
// returning their line and column positions (on the ANSI-stripped text).
func findMatches(content, term string) []matchInfo {
	if term == "" {
		return nil
	}

	lowerTerm := strings.ToLower(term)
	lines := strings.Split(content, "\n")
	var matches []matchInfo

	for lineIdx, line := range lines {
		stripped := strings.ToLower(stripAnsiCodes(line))
		start := 0
		for {
			idx := strings.Index(stripped[start:], lowerTerm)
			if idx < 0 {
				break
			}
			matches = append(matches, matchInfo{Line: lineIdx, Column: start + idx})
			start += idx + 1
		}
	}

	return matches
}

// highlightContent applies uniform highlight to all matches of term in content.
func highlightContent(content, term string) string {
	if term == "" {
		return content
	}
	matches := findMatches(content, term)
	return highlightContentWithFocus(content, term, matches, -1)
}

// highlightContentWithFocus highlights all matches, applying a distinct focused
// style to the match at focusIndex (-1 means no focus).
func highlightContentWithFocus(content, term string, matches []matchInfo, focusIndex int) string {
	if term == "" || len(matches) == 0 {
		return content
	}

	lowerTerm := strings.ToLower(term)
	termLen := len(lowerTerm)
	lines := strings.Split(content, "\n")

	lineMatches := make(map[int][]colMatch)
	for i, m := range matches {
		lineMatches[m.Line] = append(lineMatches[m.Line], colMatch{col: m.Column, focused: i == focusIndex})
	}

	var result []string
	for lineIdx, line := range lines {
		cms, ok := lineMatches[lineIdx]
		if !ok {
			result = append(result, line)
			continue
		}
		result = append(result, highlightLine(line, lowerTerm, termLen, cms))
	}

	return strings.Join(result, "\n")
}

// highlightLine inserts ANSI highlight codes into a single line for the given matches.
func highlightLine(line, lowerTerm string, termLen int, cms []colMatch) string {
	indexMap := buildIndexMapping(line)
	stripped := stripAnsiCodes(line)
	lowerStripped := strings.ToLower(stripped)

	type insertion struct {
		origPos int
		code    string
	}
	var insertions []insertion

	for _, cm := range cms {
		if cm.col+termLen > len(lowerStripped) {
			continue
		}
		if lowerStripped[cm.col:cm.col+termLen] != lowerTerm {
			continue
		}

		startOrig := indexMap[cm.col]
		endOrig := indexMap[cm.col+termLen]

		if cm.focused {
			insertions = append(insertions, insertion{origPos: startOrig, code: focusOn})
			insertions = append(insertions, insertion{origPos: endOrig, code: focusOff})
		} else {
			insertions = append(insertions, insertion{origPos: startOrig, code: highlightOn})
			insertions = append(insertions, insertion{origPos: endOrig, code: highlightOff})
		}
	}

	if len(insertions) == 0 {
		return line
	}

	// Sort descending by position so inserts don't shift later offsets
	for i := 0; i < len(insertions); i++ {
		for j := i + 1; j < len(insertions); j++ {
			if insertions[j].origPos > insertions[i].origPos {
				insertions[i], insertions[j] = insertions[j], insertions[i]
			}
		}
	}

	result := line
	for _, ins := range insertions {
		if ins.origPos > len(result) {
			ins.origPos = len(result)
		}
		result = result[:ins.origPos] + ins.code + result[ins.origPos:]
	}

	return result
}

// buildIndexMapping creates a mapping from stripped-text index to original-text index.
func buildIndexMapping(line string) []int {
	stripped := stripAnsiCodes(line)
	indexMap := make([]int, len(stripped)+1)

	origIdx := 0
	strippedIdx := 0

	for origIdx < len(line) && strippedIdx < len(stripped) {
		if line[origIdx] == '\x1b' {
			loc := ansiPattern.FindStringIndex(line[origIdx:])
			if loc != nil && loc[0] == 0 {
				origIdx += loc[1]
				continue
			}
		}
		indexMap[strippedIdx] = origIdx
		origIdx++
		strippedIdx++
	}
	indexMap[strippedIdx] = origIdx

	return indexMap
}

// lineForMatch returns the line number for the match at the given index.
func lineForMatch(matches []matchInfo, index int) int {
	if len(matches) == 0 || index < 0 || index >= len(matches) {
		return 0
	}
	return matches[index].Line
}
