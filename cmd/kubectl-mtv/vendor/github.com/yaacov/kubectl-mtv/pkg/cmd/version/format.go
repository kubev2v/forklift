package version

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// FormatOutput formats the version information according to the specified format
func (info Info) FormatOutput(format string) (string, error) {
	switch format {
	case "json":
		return info.formatJSON()
	case "yaml":
		return info.formatYAML()
	default:
		return info.formatTable(), nil
	}
}

// formatJSON returns JSON formatted version information
func (info Info) formatJSON() (string, error) {
	jsonBytes, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshaling JSON: %w", err)
	}
	return string(jsonBytes), nil
}

// formatYAML returns YAML formatted version information
func (info Info) formatYAML() (string, error) {
	yamlBytes, err := yaml.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("error marshaling YAML: %w", err)
	}
	return string(yamlBytes), nil
}

// formatTable returns table/text formatted version information
func (info Info) formatTable() string {
	output := fmt.Sprintf("kubectl-mtv version: %s\n", info.ClientVersion)

	// Operator information - use status to decide how to print
	if info.OperatorStatus == "installed" {
		output += fmt.Sprintf("MTV Operator: %s\n", info.OperatorVersion)
		output += fmt.Sprintf("MTV Namespace: %s\n", info.OperatorNamespace)
	} else {
		output += fmt.Sprintf("MTV Operator: %s\n", info.OperatorStatus)
	}

	// Inventory information - combine URL and status
	if info.InventoryStatus == "available" {
		output += fmt.Sprintf("MTV Inventory: %s\n", info.InventoryURL)
	} else {
		output += fmt.Sprintf("MTV Inventory: %s\n", info.InventoryStatus)
	}

	return output
}
