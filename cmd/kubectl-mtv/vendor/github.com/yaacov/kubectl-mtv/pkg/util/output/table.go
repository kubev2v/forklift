package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/yaacov/kubectl-mtv/pkg/util/query"
)

// Column describes a single table column: its header text, the map key used
// to extract cell values, and an optional colorizer applied to cell values.
type Column struct {
	Title     string
	Key       string
	ColorFunc func(string) string
}

// TablePrinter builds tabular data from []map[string]interface{} rows and
// renders them via lipgloss/table.
type TablePrinter struct {
	columns       []Column
	items         []map[string]interface{}
	writer        io.Writer
	selectOptions []query.SelectOption
}

// NewTablePrinter creates a new TablePrinter that writes to stdout.
func NewTablePrinter() *TablePrinter {
	return &TablePrinter{
		columns: []Column{},
		items:   []map[string]interface{}{},
		writer:  os.Stdout,
	}
}

// WithColumns sets the table columns.
func (t *TablePrinter) WithColumns(cols ...Column) *TablePrinter {
	t.columns = cols
	return t
}

// WithWriter sets the output writer (default: os.Stdout).
func (t *TablePrinter) WithWriter(w io.Writer) *TablePrinter {
	t.writer = w
	return t
}

// WithSelectOptions sets select options for advanced value extraction.
func (t *TablePrinter) WithSelectOptions(opts []query.SelectOption) *TablePrinter {
	t.selectOptions = opts
	return t
}

// AddItem appends a single row to the table.
func (t *TablePrinter) AddItem(item map[string]interface{}) *TablePrinter {
	t.items = append(t.items, item)
	return t
}

// AddItems appends multiple rows to the table.
func (t *TablePrinter) AddItems(items []map[string]interface{}) *TablePrinter {
	t.items = append(t.items, items...)
	return t
}

// PrintEmpty prints a message when there are no items to display.
func (t *TablePrinter) PrintEmpty(message string) error {
	_, err := fmt.Fprintln(t.writer, message)
	return err
}

// Print renders the table to the configured writer.
func (t *TablePrinter) Print() error {
	headers, rows := t.buildTable()
	if len(headers) == 0 {
		return nil
	}

	tbl := table.New().
		Headers(headers...).
		Rows(rows...).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderHeader(true).
		BorderStyle(lipgloss.NewStyle()).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().PaddingRight(2)
			if row == table.HeaderRow && IsColorEnabled() {
				return s.Bold(true)
			}
			return s
		})

	_, err := fmt.Fprintln(t.writer, tbl.Render())
	return err
}

// PrintMarkdown renders the table as a GitHub-flavored markdown table.
// ANSI color codes are stripped from all cell values.
func (t *TablePrinter) PrintMarkdown() error {
	if len(t.columns) == 0 {
		return nil
	}

	headers := make([]string, len(t.columns))
	for i, c := range t.columns {
		headers[i] = c.Title
	}

	if _, err := fmt.Fprintln(t.writer, "| "+strings.Join(headers, " | ")+" |"); err != nil {
		return err
	}

	sep := make([]string, len(t.columns))
	for i := range sep {
		sep[i] = "---"
	}
	if _, err := fmt.Fprintln(t.writer, "| "+strings.Join(sep, " | ")+" |"); err != nil {
		return err
	}

	for _, item := range t.items {
		cells := make([]string, len(t.columns))
		for j, c := range t.columns {
			val := StripANSI(t.extractValue(item, c.Key))
			cells[j] = strings.ReplaceAll(val, "|", `\|`)
		}
		if _, err := fmt.Fprintln(t.writer, "| "+strings.Join(cells, " | ")+" |"); err != nil {
			return err
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Convenience functions for common output patterns
// ---------------------------------------------------------------------------

// PrintTableWithQuery prints the given data as a table using TablePrinter,
// supporting dynamic columns from query options and empty message handling.
func PrintTableWithQuery(data interface{}, defaultColumns []Column, queryOpts *query.QueryOptions, emptyMessage string) error {
	items, err := toItemSlice(data)
	if err != nil {
		return err
	}

	var printer *TablePrinter

	if queryOpts != nil && queryOpts.HasSelect {
		cols := make([]Column, 0, len(queryOpts.Select))
		for _, sel := range queryOpts.Select {
			display := sel.Alias
			if display == "" {
				display = strings.TrimPrefix(sel.Field, ".")
			}
			cols = append(cols, Column{
				Title: display,
				Key:   display,
			})
		}
		printer = NewTablePrinter().
			WithColumns(cols...).
			WithSelectOptions(queryOpts.Select)
	} else {
		printer = NewTablePrinter().
			WithColumns(defaultColumns...)
	}

	if len(items) == 0 && emptyMessage != "" {
		return printer.PrintEmpty(emptyMessage)
	}

	printer.AddItems(items)
	return printer.Print()
}

// PrintMarkdownWithQuery prints data as a markdown table, supporting dynamic
// columns from query options and empty message handling.
func PrintMarkdownWithQuery(data interface{}, defaultColumns []Column, queryOpts *query.QueryOptions, emptyMessage string) error {
	items, err := toItemSlice(data)
	if err != nil {
		return err
	}

	var printer *TablePrinter

	if queryOpts != nil && queryOpts.HasSelect {
		cols := make([]Column, 0, len(queryOpts.Select))
		for _, sel := range queryOpts.Select {
			display := sel.Alias
			if display == "" {
				display = strings.TrimPrefix(sel.Field, ".")
			}
			cols = append(cols, Column{
				Title: display,
				Key:   display,
			})
		}
		printer = NewTablePrinter().
			WithColumns(cols...).
			WithSelectOptions(queryOpts.Select)
	} else {
		printer = NewTablePrinter().
			WithColumns(defaultColumns...)
	}

	if len(items) == 0 && emptyMessage != "" {
		return printer.PrintEmpty(emptyMessage)
	}

	printer.AddItems(items)
	return printer.PrintMarkdown()
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// buildTable extracts header strings and row string slices from the configured
// columns and items. ColorFunc is applied to cell values.
func (t *TablePrinter) buildTable() ([]string, [][]string) {
	headers := make([]string, len(t.columns))
	for i, c := range t.columns {
		headers[i] = c.Title
	}

	rows := make([][]string, 0, len(t.items))
	for _, item := range t.items {
		row := make([]string, len(t.columns))
		for j, c := range t.columns {
			val := t.extractValue(item, c.Key)
			if c.ColorFunc != nil {
				val = c.ColorFunc(val)
			}
			row[j] = val
		}
		rows = append(rows, row)
	}

	return headers, rows
}

// extractValue extracts a value from an item using a map key.
func (t *TablePrinter) extractValue(item map[string]interface{}, key string) string {
	if key == "" {
		return ""
	}

	if len(t.selectOptions) > 0 {
		val, err := query.GetValue(item, key, t.selectOptions)
		if err != nil {
			return ""
		}
		return valueToString(val)
	}

	value, err := query.GetValueByPathString(item, key)
	if err != nil {
		return ""
	}

	return valueToString(value)
}

// valueToString converts a value of any supported type to a string.
func valueToString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case float32:
		return fmt.Sprintf("%g", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// toItemSlice normalizes various data shapes into []map[string]interface{}.
func toItemSlice(data interface{}) ([]map[string]interface{}, error) {
	if items, ok := data.([]map[string]interface{}); ok {
		return items, nil
	}
	if item, ok := data.(map[string]interface{}); ok {
		return []map[string]interface{}{item}, nil
	}
	if slice, ok := data.([]interface{}); ok {
		items := make([]map[string]interface{}, len(slice))
		for i, elem := range slice {
			if m, ok := elem.(map[string]interface{}); ok {
				items[i] = m
			} else {
				return nil, fmt.Errorf("unsupported data type for table output: slice contains non-map elements")
			}
		}
		return items, nil
	}
	return nil, fmt.Errorf("unsupported data type for table output")
}
