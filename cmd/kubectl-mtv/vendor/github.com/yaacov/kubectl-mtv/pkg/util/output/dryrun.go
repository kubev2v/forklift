package output

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/yaml"
)

// OutputResource serializes a Kubernetes resource object to the specified format
// and prints it to stdout. Supports "yaml" (default) and "json" formats.
// YAML output is prefixed with "---\n" for multi-document compatibility.
// Call multiple times to output multiple resources (e.g., Secret + Provider).
func OutputResource(obj interface{}, format string) error {
	if obj == nil {
		return fmt.Errorf("resource object is nil")
	}

	switch format {
	case "yaml", "":
		data, err := yaml.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to marshal resource to YAML: %v", err)
		}
		fmt.Print("---\n")
		fmt.Print(string(data))
		return nil
	case "json":
		data, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal resource to JSON: %v", err)
		}
		fmt.Println(string(data))
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s. Valid formats are: json, yaml", format)
	}
}
