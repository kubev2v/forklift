package inventory

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
)

// ListJobTemplates fetches AAP job templates from the inventory service and displays them.
func ListJobTemplates(ctx context.Context, configFlags *genericclioptions.ConfigFlags, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	data, err := client.FetchAAPJobTemplatesWithInsecure(ctx, configFlags, inventoryURL, insecureSkipTLS)
	if err != nil {
		return fmt.Errorf("failed to fetch AAP job templates: %v", err)
	}

	// The response is {"count": N, "results": [...]}
	responseMap, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected response format: expected object with count and results")
	}

	resultsRaw, ok := responseMap["results"]
	if !ok {
		return fmt.Errorf("unexpected response format: missing results field")
	}

	resultsArray, ok := resultsRaw.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected response format: results is not an array")
	}

	items := make([]map[string]interface{}, 0, len(resultsArray))
	for _, item := range resultsArray {
		if m, ok := item.(map[string]interface{}); ok {
			items = append(items, m)
		}
	}

	queryOpts, err := querypkg.ParseQueryString(query)
	if err != nil {
		return fmt.Errorf("invalid query string: %v", err)
	}

	items, err = querypkg.ApplyQuery(items, queryOpts)
	if err != nil {
		return fmt.Errorf("error applying query: %v", err)
	}

	outputFormat = strings.ToLower(outputFormat)
	if outputFormat != "table" && outputFormat != "json" && outputFormat != "yaml" && outputFormat != "markdown" {
		return fmt.Errorf("unsupported output format: %s. Supported formats: table, json, yaml, markdown", outputFormat)
	}

	headers := []output.Column{
		{Title: "ID", Key: "id"},
		{Title: "NAME", Key: "name"},
	}

	emptyMessage := "No AAP job templates found (is AAP configured on ForkliftController?)"
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(items, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(items, emptyMessage)
	case "markdown":
		return output.PrintMarkdownWithQuery(items, headers, queryOpts, emptyMessage)
	default:
		return output.PrintTableWithQuery(items, headers, queryOpts, emptyMessage)
	}
}
