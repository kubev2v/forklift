package plan

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// FormatTime formats a timestamp string with optional UTC conversion
func FormatTime(timestamp string, useUTC bool) string {
	if timestamp == "" {
		return "N/A"
	}

	// Parse the timestamp
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return timestamp
	}

	// Convert to UTC or local time as requested
	if useUTC {
		t = t.UTC()
	} else {
		t = t.Local()
	}

	// Format as "2006-01-02 15:04:05"
	return t.Format("2006-01-02 15:04:05")
}

// PrintTable prints a table with headers and rows
func PrintTable(headers []string, rows [][]string, colWidths []int) {
	if len(headers) == 0 {
		return
	}

	// Calculate optimal column widths
	widths := calculateColumnWidths(headers, rows, colWidths)

	// Print headers
	printTableRow(headers, widths, 2)

	// Print separator line
	printTableSeparator(widths, 2)

	// Print data rows
	for _, row := range rows {
		printTableRow(row, widths, 2)
	}
}

// calculateColumnWidths determines optimal width for each column
func calculateColumnWidths(headers []string, rows [][]string, colWidths []int) []int {
	widths := make([]int, len(headers))

	// If specific column widths are provided, use them directly
	if len(colWidths) == len(headers) {
		copy(widths, colWidths)
		return widths
	}

	// Initialize with minimum widths
	minWidth := 8
	for i := range widths {
		widths[i] = minWidth
	}

	// Check header widths
	for i, header := range headers {
		headerLen := utf8.RuneCountInString(stripAnsiCodes(header))
		if headerLen > widths[i] {
			widths[i] = headerLen
		}
	}

	// Check data cell widths
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				cellLen := utf8.RuneCountInString(stripAnsiCodes(cell))
				if cellLen > widths[i] {
					widths[i] = cellLen
				}
			}
		}
	}

	return widths
}

// printTableRow prints a single row with proper alignment
func printTableRow(row []string, widths []int, padding int) {
	for i := 0; i < len(widths); i++ {
		var cell string
		if i < len(row) {
			cell = row[i]
		}
		// If row has fewer cells than headers, cell will be empty string

		// Truncate cell if it exceeds the column width
		displayCell := truncateCell(cell, widths[i])

		// Print cell with proper alignment considering ANSI color codes
		printCellWithAlignment(displayCell, widths[i])

		// Add padding between columns (except for last column)
		if i < len(widths)-1 {
			fmt.Print(strings.Repeat(" ", padding))
		}
	}
	fmt.Println()
}

// printCellWithAlignment prints a cell with correct alignment accounting for ANSI escape codes
func printCellWithAlignment(cell string, width int) {
	// Calculate visual length (without ANSI codes) for proper spacing
	stripped := stripAnsiCodes(cell)
	visualLen := utf8.RuneCountInString(stripped)

	// Print the cell content with ANSI colors preserved
	fmt.Print(cell)

	// Pad with spaces to reach the target column width
	if visualLen < width {
		spacesNeeded := width - visualLen
		fmt.Print(strings.Repeat(" ", spacesNeeded))
	}
}

// printTableSeparator prints the separator line between headers and data
func printTableSeparator(widths []int, padding int) {
	for i, width := range widths {
		fmt.Print(strings.Repeat("-", width))

		// Add padding between columns (except for last column)
		if i < len(widths)-1 {
			fmt.Print(strings.Repeat(" ", padding))
		}
	}
	fmt.Println()
}

// truncateCell truncates a cell to fit within the specified width
func truncateCell(cell string, maxWidth int) string {
	// Strip ANSI codes for length calculation, but preserve them in output
	stripped := stripAnsiCodes(cell)

	if utf8.RuneCountInString(stripped) <= maxWidth {
		return cell
	}

	// Handle text containing ANSI color codes
	if len(cell) != len(stripped) {
		// Preserve ANSI codes while truncating text content
		if maxWidth > 3 {
			// Conservative truncation for colored text
			return cell[:min(len(cell), maxWidth*2)]
		}
		return cell[:min(len(cell), maxWidth)]
	}

	// Plain text - safe to truncate
	if maxWidth > 3 {
		runes := []rune(cell)
		return string(runes[:maxWidth-3]) + "..."
	}

	runes := []rune(cell)
	return string(runes[:maxWidth])
}

// stripAnsiCodes removes ANSI escape sequences for length calculation
func stripAnsiCodes(s string) string {
	result := strings.Builder{}
	inEscape := false

	for _, r := range s {
		if r == '\033' { // Start of ANSI escape sequence
			inEscape = true
			continue
		}

		if inEscape {
			if r == 'm' { // End of color code
				inEscape = false
			}
			continue
		}

		result.WriteRune(r)
	}

	return result.String()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
