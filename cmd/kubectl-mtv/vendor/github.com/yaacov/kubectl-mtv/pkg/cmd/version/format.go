package version

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// escapeMarkdownCell escapes characters that break markdown table layout.
func escapeMarkdownCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", "<br>")
	return s
}

// FormatOutput formats the version information according to the specified format
func (info Info) FormatOutput(format string) (string, error) {
	switch format {
	case "json":
		return info.formatJSON()
	case "yaml":
		return info.formatYAML()
	case "markdown":
		return info.formatMarkdown(), nil
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

// formatMarkdown returns markdown formatted version information
func (info Info) formatMarkdown() string {
	out := "| Field | Value |\n|---|---|\n"
	out += fmt.Sprintf("| kubectl-mtv version | %s |\n", escapeMarkdownCell(info.ClientVersion))

	if info.OperatorStatus != "" {
		if info.OperatorStatus == "installed" {
			out += fmt.Sprintf("| MTV Operator | %s |\n", escapeMarkdownCell(info.OperatorVersion))
			out += fmt.Sprintf("| MTV Namespace | %s |\n", escapeMarkdownCell(info.OperatorNamespace))
		} else {
			out += fmt.Sprintf("| MTV Operator | %s |\n", escapeMarkdownCell(info.OperatorStatus))
		}
	}

	if info.InventoryStatus != "" {
		if info.InventoryStatus == "available" {
			url := info.InventoryURL
			if info.InventoryInsecure {
				url += " (insecure)"
			}
			out += fmt.Sprintf("| MTV Inventory | %s |\n", escapeMarkdownCell(url))
		} else {
			out += fmt.Sprintf("| MTV Inventory | %s |\n", escapeMarkdownCell(info.InventoryStatus))
		}
	}

	return out
}

// formatTable returns table/text formatted version information
func (info Info) formatTable() string {
	output := fmt.Sprintf("kubectl-mtv version: %s\n", info.ClientVersion)

	// Operator information - only print if we have status
	if info.OperatorStatus != "" {
		if info.OperatorStatus == "installed" {
			output += fmt.Sprintf("MTV Operator: %s\n", info.OperatorVersion)
			output += fmt.Sprintf("MTV Namespace: %s\n", info.OperatorNamespace)
		} else {
			output += fmt.Sprintf("MTV Operator: %s\n", info.OperatorStatus)
		}
	}

	// Inventory information - only print if we have status
	if info.InventoryStatus != "" {
		if info.InventoryStatus == "available" {
			if info.InventoryInsecure {
				output += fmt.Sprintf("MTV Inventory: %s (insecure)\n", info.InventoryURL)
			} else {
				output += fmt.Sprintf("MTV Inventory: %s\n", info.InventoryURL)
			}
		} else {
			output += fmt.Sprintf("MTV Inventory: %s\n", info.InventoryStatus)
		}
	}

	return output
}
