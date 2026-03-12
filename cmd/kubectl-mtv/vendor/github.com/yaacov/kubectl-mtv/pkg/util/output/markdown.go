package output

import (
	"fmt"
	"strings"

	"github.com/yaacov/kubectl-mtv/pkg/util/query"
)

// PrintMarkdownWithQuery prints data as a markdown table, supporting dynamic
// headers from query options and empty message handling.
func PrintMarkdownWithQuery(data interface{}, defaultHeaders []Header, queryOpts *query.QueryOptions, emptyMessage string) error {
	items, ok := data.([]map[string]interface{})
	if !ok {
		if item, ok := data.(map[string]interface{}); ok {
			items = []map[string]interface{}{item}
		} else if slice, ok := data.([]interface{}); ok {
			items = make([]map[string]interface{}, len(slice))
			for i, item := range slice {
				if mapItem, ok := item.(map[string]interface{}); ok {
					items[i] = mapItem
				} else {
					return fmt.Errorf("unsupported data type for markdown output: slice contains non-map elements")
				}
			}
		} else {
			return fmt.Errorf("unsupported data type for markdown output")
		}
	}

	var printer *TablePrinter

	if queryOpts != nil && queryOpts.HasSelect {
		headers := make([]Header, 0, len(queryOpts.Select))
		for _, sel := range queryOpts.Select {
			display := sel.Alias
			if display == "" {
				display = strings.TrimPrefix(sel.Field, ".")
			}
			headers = append(headers, Header{
				DisplayName: display,
				JSONPath:    display,
			})
		}
		printer = NewTablePrinter().
			WithHeaders(headers...).
			WithSelectOptions(queryOpts.Select)
	} else {
		printer = NewTablePrinter().
			WithHeaders(defaultHeaders...)
	}

	if len(items) == 0 && emptyMessage != "" {
		return printer.PrintEmpty(emptyMessage)
	}

	printer.AddItems(items)
	return printer.PrintMarkdown()
}

// PrintMarkdown renders the table as a GitHub-flavored markdown table.
// ANSI color codes are stripped from all cell values.
func (t *TablePrinter) PrintMarkdown() error {
	if len(t.headers) == 0 {
		return nil
	}

	// Header row
	t.printMarkdownRow(t.headerNames())

	// Separator row
	sep := make([]string, len(t.headers))
	for i := range sep {
		sep[i] = "---"
	}
	fmt.Fprintln(t.writer, "| "+strings.Join(sep, " | ")+" |")

	// Data rows
	for _, item := range t.items {
		row := make([]string, len(t.headers))
		for j, header := range t.headers {
			row[j] = StripANSI(t.extractValue(item, header.JSONPath))
		}
		t.printMarkdownRow(row)
	}

	return nil
}

// headerNames returns the display names of all headers.
func (t *TablePrinter) headerNames() []string {
	names := make([]string, len(t.headers))
	for i, h := range t.headers {
		names[i] = h.DisplayName
	}
	return names
}

// printMarkdownRow writes a single pipe-delimited markdown row, escaping any
// literal pipe characters inside cell values.
func (t *TablePrinter) printMarkdownRow(cells []string) {
	escaped := make([]string, len(cells))
	for i, c := range cells {
		escaped[i] = escapeMarkdownPipe(c)
	}
	fmt.Fprintln(t.writer, "| "+strings.Join(escaped, " | ")+" |")
}

// escapeMarkdownPipe replaces literal | with \| so markdown parsers don't
// treat them as column delimiters.
func escapeMarkdownPipe(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}
