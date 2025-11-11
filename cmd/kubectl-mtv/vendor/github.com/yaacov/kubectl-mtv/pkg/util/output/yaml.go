package output

// PrintYAML prints the given data as YAML using YAMLPrinter
import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// PrintYAMLWithEmpty prints the given data as YAML using YAMLPrinter with empty handling
func PrintYAMLWithEmpty(data interface{}, emptyMessage string) error {
	items, ok := data.([]map[string]interface{})
	printer := NewYAMLPrinter()

	if ok {
		if len(items) == 0 && emptyMessage != "" {
			return printer.PrintEmpty(emptyMessage)
		}
		printer.AddItems(items)
	} else if item, ok := data.(map[string]interface{}); ok {
		printer.AddItem(item)
	} else {
		// Fallback: marshal any data
		b, err := yaml.Marshal(data)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout, string(b))
		return err
	}
	return printer.Print()
}

// YAMLPrinter prints data as YAML
type YAMLPrinter struct {
	items       []map[string]interface{}
	writer      io.Writer
	prettyPrint bool
}

// NewYAMLPrinter creates a new YAMLPrinter
func NewYAMLPrinter() *YAMLPrinter {
	return &YAMLPrinter{
		items:       []map[string]interface{}{},
		writer:      os.Stdout,
		prettyPrint: true, // YAML is inherently formatted, so default to true
	}
}

// WithWriter sets the output writer
func (y *YAMLPrinter) WithWriter(writer io.Writer) *YAMLPrinter {
	y.writer = writer
	return y
}

// WithPrettyPrint enables or disables pretty printing (indentation)
// Note: YAML is inherently formatted, so this mainly affects structure
func (y *YAMLPrinter) WithPrettyPrint(pretty bool) *YAMLPrinter {
	y.prettyPrint = pretty
	return y
}

// AddItem adds an item to the YAML output
func (y *YAMLPrinter) AddItem(item map[string]interface{}) *YAMLPrinter {
	y.items = append(y.items, item)
	return y
}

// AddItems adds multiple items to the YAML output
func (y *YAMLPrinter) AddItems(items []map[string]interface{}) *YAMLPrinter {
	y.items = append(y.items, items...)
	return y
}

// Print outputs the items as YAML
func (y *YAMLPrinter) Print() error {
	encoder := yaml.NewEncoder(y.writer)
	encoder.SetIndent(2)

	defer encoder.Close()

	err := encoder.Encode(y.items)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %v", err)
	}

	return nil
}

// PrintEmpty outputs an empty YAML array or a message when there are no items
func (y *YAMLPrinter) PrintEmpty(message string) error {
	encoder := yaml.NewEncoder(y.writer)
	encoder.SetIndent(2)

	defer encoder.Close()

	var data interface{}

	if message == "" {
		// If no message, just print an empty array
		data = []interface{}{}
	} else {
		// If message provided, print only the message
		data = map[string]interface{}{
			"message": message,
		}
	}

	err := encoder.Encode(data)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %v", err)
	}

	return nil
}
