package output

// PrintTable prints the given data as a table using TablePrinter and headers
import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yaacov/kubectl-mtv/pkg/util/query"
)

// PrintTableWithQuery prints the given data as a table using TablePrinter,
// supporting dynamic headers from query options and empty message handling.
func PrintTableWithQuery(data interface{}, defaultHeaders []Header, queryOpts *query.QueryOptions, emptyMessage string) error {
	items, ok := data.([]map[string]interface{})
	if !ok {
		if item, ok := data.(map[string]interface{}); ok {
			// Handle single item map
			items = []map[string]interface{}{item}
		} else if slice, ok := data.([]interface{}); ok {
			// Handle []interface{} from JSON unmarshaling
			items = make([]map[string]interface{}, len(slice))
			for i, item := range slice {
				if mapItem, ok := item.(map[string]interface{}); ok {
					items[i] = mapItem
				} else {
					return fmt.Errorf("unsupported data type for table output: slice contains non-map elements")
				}
			}
		} else {
			return fmt.Errorf("unsupported data type for table output")
		}
	}

	var printer *TablePrinter

	// Check if we should use custom headers from SELECT clause
	if queryOpts != nil && queryOpts.HasSelect {
		headers := make([]Header, 0, len(queryOpts.Select))
		for _, sel := range queryOpts.Select {
			headers = append(headers, Header{
				DisplayName: sel.Alias,
				JSONPath:    sel.Alias,
			})
		}
		printer = NewTablePrinter().
			WithHeaders(headers...).
			WithSelectOptions(queryOpts.Select).
			WithJSONPathRow().
			WithSeparator("─")
	} else {
		// Use the provided default headers
		printer = NewTablePrinter().
			WithHeaders(defaultHeaders...).
			WithJSONPathRow().
			WithSeparator("─")
	}

	if len(items) == 0 && emptyMessage != "" {
		return printer.PrintEmpty(emptyMessage)
	}

	printer.AddItems(items)
	return printer.Print()
}

// Header represents a table column header with display text and a JSON path
type Header struct {
	DisplayName string
	JSONPath    string
	ColorFunc   func(string) string
}

// TablePrinter prints tabular data with dynamically sized columns
type TablePrinter struct {
	headers         []Header
	items           []map[string]interface{}
	padding         int
	minWidth        int
	writer          io.Writer
	maxColWidth     int
	expandedData    map[int]string       // Stores expanded data for each row by index
	selectOptions   []query.SelectOption // Optional: select options for advanced extraction
	separator       string               // if set, printed between header and data rows
	columnWidths    []int                // if set, overrides auto-calculated widths
	showJSONPathRow bool                 // if true, prints a row of JSON paths below the header
}

// NewTablePrinter creates a new TablePrinter
func NewTablePrinter() *TablePrinter {
	return &TablePrinter{
		headers:      []Header{},
		items:        []map[string]interface{}{},
		padding:      2,
		minWidth:     10,
		writer:       os.Stdout,
		maxColWidth:  50, // Prevent very wide columns
		expandedData: make(map[int]string),
	}
}

// WithHeaders sets the table headers with display names and JSON paths
func (t *TablePrinter) WithHeaders(headers ...Header) *TablePrinter {
	t.headers = headers
	return t
}

// WithPadding sets the padding between columns
func (t *TablePrinter) WithPadding(padding int) *TablePrinter {
	t.padding = padding
	return t
}

// WithMinWidth sets the minimum column width
func (t *TablePrinter) WithMinWidth(minWidth int) *TablePrinter {
	t.minWidth = minWidth
	return t
}

// WithMaxWidth sets the maximum column width
func (t *TablePrinter) WithMaxWidth(maxWidth int) *TablePrinter {
	t.maxColWidth = maxWidth
	return t
}

// WithWriter sets the output writer
func (t *TablePrinter) WithWriter(writer io.Writer) *TablePrinter {
	t.writer = writer
	return t
}

// WithExpandedData sets expanded data for a specific row index
func (t *TablePrinter) WithExpandedData(index int, data string) *TablePrinter {
	t.expandedData[index] = data
	return t
}

// WithSelectOptions sets the select options for the table printer
func (t *TablePrinter) WithSelectOptions(selectOptions []query.SelectOption) *TablePrinter {
	t.selectOptions = selectOptions
	return t
}

// WithSeparator sets the character used to draw a separator line between the header and data rows
func (t *TablePrinter) WithSeparator(char string) *TablePrinter {
	t.separator = char
	return t
}

// WithJSONPathRow enables printing a row of JSON paths below the header row
func (t *TablePrinter) WithJSONPathRow() *TablePrinter {
	t.showJSONPathRow = true
	return t
}

// WithColumnWidths sets explicit column widths, overriding auto-calculation
func (t *TablePrinter) WithColumnWidths(widths []int) *TablePrinter {
	t.columnWidths = widths
	return t
}

// AddItem adds an item to the table
func (t *TablePrinter) AddItem(item map[string]interface{}) *TablePrinter {
	t.items = append(t.items, item)
	return t
}

// AddItemWithExpanded adds an item to the table with expanded data
func (t *TablePrinter) AddItemWithExpanded(item map[string]interface{}, expanded string) *TablePrinter {
	index := len(t.items)
	t.items = append(t.items, item)
	t.expandedData[index] = expanded
	return t
}

// AddItems adds multiple items to the table
func (t *TablePrinter) AddItems(items []map[string]interface{}) *TablePrinter {
	t.items = append(t.items, items...)
	return t
}

// extractValue extracts a value from an item using a JSON path
func (t *TablePrinter) extractValue(item map[string]interface{}, path string) string {
	if path == "" {
		// No path provided, return empty string
		return ""
	}

	// Use query.GetValue if selectOptions are set, otherwise fallback to GetValueByPathString
	if len(t.selectOptions) > 0 {
		val, err := query.GetValue(item, path, t.selectOptions)
		if err != nil {
			return ""
		}
		return valueToString(val)
	}

	value, err := query.GetValueByPathString(item, path)
	if err != nil {
		return ""
	}

	return valueToString(value)
}

// valueToString converts a value of any supported type to a string
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
		// For other types, use default string conversion
		return fmt.Sprintf("%v", v)
	}
}

// calculateColumnWidths determines the optimal width for each column
func (t *TablePrinter) calculateColumnWidths() []int {
	numCols := len(t.headers)
	if numCols == 0 {
		return []int{}
	}

	if len(t.columnWidths) == numCols {
		return t.columnWidths
	}

	// Initialize widths with minimum values
	widths := make([]int, numCols)
	for i := range widths {
		widths[i] = t.minWidth
	}

	// Check header widths
	for i, header := range t.headers {
		headerWidth := VisibleLength(header.DisplayName)
		if headerWidth > widths[i] {
			widths[i] = min(headerWidth, t.maxColWidth)
		}
		if t.showJSONPathRow {
			pathWidth := len(header.JSONPath) + 2 // +2 for surrounding brackets
			if pathWidth > widths[i] {
				widths[i] = min(pathWidth, t.maxColWidth)
			}
		}
	}

	// Calculate row data for width determination
	for _, item := range t.items {
		for i, header := range t.headers {
			value := t.extractValue(item, header.JSONPath)
			cellWidth := VisibleLength(value)
			if cellWidth > widths[i] {
				widths[i] = min(cellWidth, t.maxColWidth)
			}
		}
	}

	return widths
}

// Print prints the table with dynamic column widths
func (t *TablePrinter) Print() error {
	widths := t.calculateColumnWidths()
	if len(widths) == 0 {
		return nil
	}

	// Print headers
	headerRow := make([]string, len(t.headers))
	for i, header := range t.headers {
		headerRow[i] = header.DisplayName
	}
	t.printRow(headerRow, widths)

	if t.showJSONPathRow {
		pathRow := make([]string, len(t.headers))
		for i, header := range t.headers {
			pathRow[i] = "[" + header.JSONPath + "]"
		}
		t.printRow(pathRow, widths)
	}

	if t.separator != "" {
		t.printSeparator(widths)
	}

	// Print item rows and expanded data if available
	for i, item := range t.items {
		row := make([]string, len(t.headers))
		for j, header := range t.headers {
			value := t.extractValue(item, header.JSONPath)
			if header.ColorFunc != nil {
				value = header.ColorFunc(value)
			}
			row[j] = value
		}
		t.printRow(row, widths)

		// Print expanded data if it exists for this row
		if expanded, exists := t.expandedData[i]; exists && expanded != "" {
			// Split expanded data into lines and add prefix
			lines := strings.Split(expanded, "\n")
			for _, line := range lines {
				fmt.Fprintf(t.writer, "  │ %s\n", line)
			}
		}
	}

	return nil
}

// PrintEmpty prints a message when there are no items to display
func (t *TablePrinter) PrintEmpty(message string) error {
	fmt.Fprintln(t.writer, message)
	return nil
}

// printRow prints a single row with the specified column widths.
// Handles ANSI color codes by using visible length for padding calculations.
func (t *TablePrinter) printRow(row []string, widths []int) {
	var sb strings.Builder

	for i, cell := range row {
		if i >= len(widths) {
			break
		}

		displayCell := cell
		visLen := VisibleLength(cell)

		if visLen > t.maxColWidth {
			displayCell = TruncateANSI(cell, t.maxColWidth)
			visLen = t.maxColWidth
		}

		targetWidth := widths[i] + t.padding
		pad := targetWidth - visLen
		if pad < 0 {
			pad = 0
		}

		sb.WriteString(displayCell)
		sb.WriteString(strings.Repeat(" ", pad))
	}

	fmt.Fprintln(t.writer, strings.TrimRight(sb.String(), " "))
}

// printSeparator prints a separator line between the header and data rows.
func (t *TablePrinter) printSeparator(widths []int) {
	var sb strings.Builder
	for i, w := range widths {
		sb.WriteString(strings.Repeat(t.separator, w))
		if i < len(widths)-1 {
			sb.WriteString(strings.Repeat(" ", t.padding))
		}
	}
	fmt.Fprintln(t.writer, sb.String())
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MappingEntryFormatter is a function type for formatting mapping entries
type MappingEntryFormatter func(entryMap map[string]interface{}, entryType string) string

// PrintMappingTable prints mapping entries in a custom table format
func PrintMappingTable(mapEntries []interface{}, formatter MappingEntryFormatter) error {
	if len(mapEntries) == 0 {
		return nil
	}

	// Calculate the maximum width for both source and destination columns based on content
	maxSourceWidth := len("SOURCE")    // minimum width (header width)
	maxDestWidth := len("DESTINATION") // minimum width (header width)

	for _, entry := range mapEntries {
		if entryMap, ok := entry.(map[string]interface{}); ok {
			// Calculate source width
			sourceText := formatter(entryMap, "source")
			sourceLines := strings.Split(sourceText, "\n")
			for _, line := range sourceLines {
				if len(line) > maxSourceWidth {
					maxSourceWidth = len(line)
				}
			}

			// Calculate destination width
			destText := formatter(entryMap, "destination")
			destLines := strings.Split(destText, "\n")
			for _, line := range destLines {
				if len(line) > maxDestWidth {
					maxDestWidth = len(line)
				}
			}
		}
	}

	// Cap the widths to prevent overly wide tables
	if maxSourceWidth > 50 {
		maxSourceWidth = 50
	}
	if maxDestWidth > 50 {
		maxDestWidth = 50
	}

	// Define column spacing
	columnSpacing := "  " // 2 spaces

	// Print table header
	headerFormat := fmt.Sprintf("%%-%ds%s%%s\n", maxSourceWidth+8, columnSpacing)
	fmt.Printf(headerFormat, Bold("SOURCE"), Bold("DESTINATION"))

	// Print separator line using calculated widths
	separatorLine := strings.Repeat("─", maxSourceWidth) + columnSpacing + strings.Repeat("─", maxDestWidth)
	fmt.Println(separatorLine)

	// Process each mapping entry
	for i, entry := range mapEntries {
		if entryMap, ok := entry.(map[string]interface{}); ok {
			sourceText := formatter(entryMap, "source")
			destText := formatter(entryMap, "destination")

			printMappingTableRow(sourceText, destText, maxSourceWidth, maxDestWidth, columnSpacing)

			// Add separator between entries (except for the last one)
			if i < len(mapEntries)-1 {
				entrySeperatorLine := strings.Repeat("─", maxSourceWidth) + columnSpacing + strings.Repeat("─", maxDestWidth)
				fmt.Println(entrySeperatorLine)
			}
		}
	}

	return nil
}

// printMappingTableRow prints a single mapping row with proper alignment for multi-line content
func printMappingTableRow(source, dest string, sourceWidth, destWidth int, columnSpacing string) {
	sourceLines := strings.Split(source, "\n")
	destLines := strings.Split(dest, "\n")

	// Determine the maximum number of lines
	maxLines := len(sourceLines)
	if len(destLines) > maxLines {
		maxLines = len(destLines)
	}

	// Print each line
	for i := 0; i < maxLines; i++ {
		var sourceLine, destLine string

		if i < len(sourceLines) {
			sourceLine = sourceLines[i]
		}
		if i < len(destLines) {
			destLine = destLines[i]
		}

		// Truncate lines if they're too long
		if len(sourceLine) > sourceWidth {
			sourceLine = sourceLine[:sourceWidth-3] + "..."
		}
		if len(destLine) > destWidth {
			destLine = destLine[:destWidth-3] + "..."
		}

		// Format and print the line with proper column widths
		rowFormat := fmt.Sprintf("%%-%ds%s%%-%ds\n", sourceWidth, columnSpacing, destWidth)
		fmt.Printf(rowFormat, sourceLine, destLine)
	}
}

// PrintConditions prints conditions information in a consistent format
func PrintConditions(conditions []interface{}) {
	if len(conditions) == 0 {
		return
	}

	fmt.Printf("%s\n", Bold("Conditions:"))
	for _, condition := range conditions {
		if condMap, ok := condition.(map[string]interface{}); ok {
			condType, _ := condMap["type"].(string)
			condStatus, _ := condMap["status"].(string)
			category, _ := condMap["category"].(string)
			message, _ := condMap["message"].(string)
			lastTransitionTime, _ := condMap["lastTransitionTime"].(string)

			fmt.Printf("  %s: %s", Bold(condType), ColorizeStatus(condStatus))
			if category != "" {
				fmt.Printf(" (%s)", ColorizeCategory(category))
			}
			fmt.Println()

			if message != "" {
				fmt.Printf("    %s\n", message)
			}
			if lastTransitionTime != "" {
				fmt.Printf("    Last Transition: %s\n", lastTransitionTime)
			}
		}
	}
}
