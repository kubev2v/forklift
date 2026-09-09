package conversion

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

func extractConversionType(conv unstructured.Unstructured) string {
	t, _, _ := unstructured.NestedString(conv.Object, "spec", "type")
	return t
}

func extractConversionVM(conv unstructured.Unstructured) string {
	name, _, _ := unstructured.NestedString(conv.Object, "spec", "vm", "name")
	if name != "" {
		return name
	}
	id, _, _ := unstructured.NestedString(conv.Object, "spec", "vm", "id")
	return id
}

func extractConversionPhase(conv unstructured.Unstructured) string {
	phase, _, _ := unstructured.NestedString(conv.Object, "status", "phase")
	if phase == "" {
		return "Pending"
	}
	return phase
}

func extractConversionStage(conv unstructured.Unstructured) string {
	stage, _, _ := unstructured.NestedString(conv.Object, "status", "stage")
	return stage
}

func createConversionItem(conv unstructured.Unstructured, useUTC bool) map[string]interface{} {
	return map[string]interface{}{
		"name":      conv.GetName(),
		"namespace": conv.GetNamespace(),
		"type":      extractConversionType(conv),
		"vm":        extractConversionVM(conv),
		"phase":     extractConversionPhase(conv),
		"stage":     extractConversionStage(conv),
		"created":   output.FormatTimestamp(conv.GetCreationTimestamp().Time, useUTC),
		"object":    conv.Object,
	}
}

// ListConversions lists conversion resources without watch functionality
func ListConversions(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace, outputFormat string, convName string, useUTC bool, query string) error {
	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	outputFormat = strings.ToLower(outputFormat)
	if outputFormat != "table" && outputFormat != "json" && outputFormat != "yaml" && outputFormat != "markdown" {
		return fmt.Errorf("unsupported output format: %s. Supported formats: table, json, yaml, markdown", outputFormat)
	}

	var allItems []map[string]interface{}

	if convName != "" {
		allItems, err = getSpecificConversion(ctx, dynamicClient, namespace, convName, useUTC)
	} else {
		allItems, err = getAllConversions(ctx, dynamicClient, namespace, useUTC)
	}
	if err != nil {
		return err
	}

	if query != "" {
		queryOpts, err := querypkg.ParseQueryString(query)
		if err != nil {
			return fmt.Errorf("failed to parse query: %v", err)
		}
		allItems, err = querypkg.ApplyQuery(allItems, queryOpts)
		if err != nil {
			return fmt.Errorf("error applying query: %v", err)
		}
	}

	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(allItems, "No conversions found.")
	case "yaml":
		return output.PrintYAMLWithEmpty(allItems, "No conversions found.")
	default:
		return printConversionOutput(allItems, outputFormat)
	}
}

func getAllConversions(ctx context.Context, dynamicClient dynamic.Interface, namespace string, useUTC bool) ([]map[string]interface{}, error) {
	list, err := dynamicClient.Resource(client.ConversionsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list conversions: %v", err)
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, conv := range list.Items {
		items = append(items, createConversionItem(conv, useUTC))
	}
	return items, nil
}

func getSpecificConversion(ctx context.Context, dynamicClient dynamic.Interface, namespace string, name string, useUTC bool) ([]map[string]interface{}, error) {
	conv, err := dynamicClient.Resource(client.ConversionsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get conversion '%s': %v", name, err)
	}
	return []map[string]interface{}{createConversionItem(*conv, useUTC)}, nil
}

func printConversionOutput(items []map[string]interface{}, outputFormat string) error {
	if len(items) == 0 {
		fmt.Println("No conversions found.")
		return nil
	}

	printer := output.NewTablePrinter()

	columns := []output.Column{
		{Title: "NAME", Key: "name"},
		{Title: "TYPE", Key: "type"},
		{Title: "VM", Key: "vm"},
		{Title: "PHASE", Key: "phase", ColorFunc: output.ColorizeStatus},
		{Title: "STAGE", Key: "stage"},
		{Title: "AGE", Key: "created"},
	}
	printer.WithColumns(columns...)

	for _, item := range items {
		printer.AddItem(map[string]interface{}{
			"name":    item["name"],
			"type":    item["type"],
			"vm":      item["vm"],
			"phase":   item["phase"],
			"stage":   item["stage"],
			"created": item["created"],
		})
	}

	if outputFormat == "markdown" {
		return printer.PrintMarkdown()
	}
	return printer.Print()
}

// List lists conversions with optional watch mode
func List(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string, watchMode bool, outputFormat string, convName string, useUTC bool, query string) error {
	return watch.WrapWithWatch(watchMode, outputFormat, func() error {
		return ListConversions(ctx, configFlags, namespace, outputFormat, convName, useUTC, query)
	}, watch.DefaultInterval)
}
