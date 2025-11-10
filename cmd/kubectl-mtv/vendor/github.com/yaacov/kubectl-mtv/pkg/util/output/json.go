package output

// PrintJSON prints the given data as JSON using JSONPrinter
import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// PrintJSONWithEmpty prints the given data as JSON using JSONPrinter with empty handling
func PrintJSONWithEmpty(data interface{}, emptyMessage string) error {
	// Convert data to []map[string]interface{} if possible
	items, ok := data.([]map[string]interface{})
	printer := NewJSONPrinter().WithPrettyPrint(true)

	if ok {
		if len(items) == 0 && emptyMessage != "" {
			return printer.PrintEmpty(emptyMessage)
		}
		printer.AddItems(items)
	} else if item, ok := data.(map[string]interface{}); ok {
		printer.AddItem(item)
	} else {
		// Fallback: marshal any data
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout, string(b))
		return err
	}
	return printer.Print()
}

// JSONPrinter prints data as JSON
type JSONPrinter struct {
	items       []map[string]interface{}
	writer      io.Writer
	prettyPrint bool
}

// NewJSONPrinter creates a new JSONPrinter
func NewJSONPrinter() *JSONPrinter {
	return &JSONPrinter{
		items:       []map[string]interface{}{},
		writer:      os.Stdout,
		prettyPrint: false,
	}
}

// WithWriter sets the output writer
func (j *JSONPrinter) WithWriter(writer io.Writer) *JSONPrinter {
	j.writer = writer
	return j
}

// WithPrettyPrint enables or disables pretty printing (indentation)
func (j *JSONPrinter) WithPrettyPrint(pretty bool) *JSONPrinter {
	j.prettyPrint = pretty
	return j
}

// AddItem adds an item to the JSON output
func (j *JSONPrinter) AddItem(item map[string]interface{}) *JSONPrinter {
	j.items = append(j.items, item)
	return j
}

// AddItems adds multiple items to the JSON output
func (j *JSONPrinter) AddItems(items []map[string]interface{}) *JSONPrinter {
	j.items = append(j.items, items...)
	return j
}

// Print outputs the items as JSON
func (j *JSONPrinter) Print() error {
	var data []byte
	var err error

	if j.prettyPrint {
		data, err = json.MarshalIndent(j.items, "", "  ")
	} else {
		data, err = json.Marshal(j.items)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	_, err = fmt.Fprintln(j.writer, string(data))
	return err
}

// PrintEmpty outputs an empty JSON array or a message when there are no items
func (j *JSONPrinter) PrintEmpty(message string) error {
	var data []byte
	var err error

	if message == "" {
		// If no message, just print an empty array
		if j.prettyPrint {
			data, err = json.MarshalIndent([]interface{}{}, "", "  ")
		} else {
			data, err = json.Marshal([]interface{}{})
		}
	} else {
		// If message provided, print only the message
		result := map[string]interface{}{
			"message": message,
		}

		if j.prettyPrint {
			data, err = json.MarshalIndent(result, "", "  ")
		} else {
			data, err = json.Marshal(result)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	_, err = fmt.Fprintln(j.writer, string(data))
	return err
}
