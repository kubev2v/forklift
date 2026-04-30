package plan

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan/status"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// getPlans retrieves all plans from the given namespace
func getPlans(ctx context.Context, dynamicClient dynamic.Interface, namespace string) (*unstructured.UnstructuredList, error) {
	if namespace != "" {
		return dynamicClient.Resource(client.PlansGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		return dynamicClient.Resource(client.PlansGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	}
}

// getSpecificPlan retrieves a specific plan by name
func getSpecificPlan(ctx context.Context, dynamicClient dynamic.Interface, namespace, planName string) (*unstructured.UnstructuredList, error) {
	if namespace != "" {
		// If namespace is specified, get the specific resource
		plan, err := dynamicClient.Resource(client.PlansGVR).Namespace(namespace).Get(ctx, planName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		// Create a list with just this plan
		return &unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{*plan},
		}, nil
	} else {
		// If no namespace specified, list all and filter by name
		plans, err := dynamicClient.Resource(client.PlansGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list plans: %v", err)
		}

		var filteredItems []unstructured.Unstructured
		for _, plan := range plans.Items {
			if plan.GetName() == planName {
				filteredItems = append(filteredItems, plan)
			}
		}

		if len(filteredItems) == 0 {
			return nil, fmt.Errorf("plan '%s' not found", planName)
		}

		return &unstructured.UnstructuredList{
			Items: filteredItems,
		}, nil
	}
}

// ListPlans lists migration plans without watch functionality
func ListPlans(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string, outputFormat string, planName string, useUTC bool, query string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	var plans *unstructured.UnstructuredList
	if planName != "" {
		// Get specific plan by name
		plans, err = getSpecificPlan(ctx, c, namespace, planName)
		if err != nil {
			return fmt.Errorf("failed to get plan: %v", err)
		}
	} else {
		// Get all plans
		plans, err = getPlans(ctx, c, namespace)
		if err != nil {
			return fmt.Errorf("failed to list plans: %v", err)
		}
	}

	// Format validation
	outputFormat = strings.ToLower(outputFormat)
	if outputFormat != "table" && outputFormat != "json" && outputFormat != "yaml" && outputFormat != "markdown" {
		return fmt.Errorf("unsupported output format: %s. Supported formats: table, json, yaml, markdown", outputFormat)
	}

	// Create printer items
	items := []map[string]interface{}{}
	for _, p := range plans.Items {
		source, _, _ := unstructured.NestedString(p.Object, "spec", "provider", "source", "name")
		target, _, _ := unstructured.NestedString(p.Object, "spec", "provider", "destination", "name")
		vms, _, _ := unstructured.NestedSlice(p.Object, "spec", "vms")
		creationTime := p.GetCreationTimestamp()

		// Get archived status
		archived, exists, _ := unstructured.NestedBool(p.Object, "spec", "archived")
		if !exists {
			archived = false
		}

		// Get plan details (ready, running migration, status)
		planDetails, _ := status.GetPlanDetails(c, namespace, &p, client.MigrationsGVR)

		// Format the VM migration status
		var vmStatus string
		if planDetails.RunningMigration != nil && planDetails.VMStats.Total > 0 {
			vmStatus = fmt.Sprintf("%d/%d (S:%d/F:%d/C:%d)",
				planDetails.VMStats.Completed,
				planDetails.VMStats.Total,
				planDetails.VMStats.Succeeded,
				planDetails.VMStats.Failed,
				planDetails.VMStats.Canceled)
		} else {
			vmStatus = fmt.Sprintf("%d", len(vms))
		}

		// Format the disk transfer progress
		progressStatus := "-"
		if planDetails.RunningMigration != nil && planDetails.DiskProgress.Total > 0 {
			percentage := float64(planDetails.DiskProgress.Completed) / float64(planDetails.DiskProgress.Total) * 100
			progressStatus = fmt.Sprintf("%.1f%% (%d/%d GB)",
				percentage,
				planDetails.DiskProgress.Completed/(1024), // Convert to GB
				planDetails.DiskProgress.Total/(1024))     // Convert to GB
		}

		// Determine migration type and cutover information
		cutoverInfo := "cold" // Default for cold migration

		// First check the new 'type' field
		migrationType, exists, _ := unstructured.NestedString(p.Object, "spec", "type")
		if exists && migrationType != "" {
			cutoverInfo = migrationType
		} else {
			// Fall back to legacy 'warm' boolean field
			warm, exists, _ := unstructured.NestedBool(p.Object, "spec", "warm")
			if exists && warm {
				cutoverInfo = "warm"
			}
		}

		// For warm migrations, check if there's a specific cutover time
		if cutoverInfo == "warm" && planDetails.RunningMigration != nil {
			// Extract cutover time from running migration
			cutoverTimeStr, exists, _ := unstructured.NestedString(planDetails.RunningMigration.Object, "spec", "cutover")
			if exists && cutoverTimeStr != "" {
				// Parse the cutover time string
				cutoverTime, err := time.Parse(time.RFC3339, cutoverTimeStr)
				if err == nil {
					cutoverInfo = output.FormatTimestamp(cutoverTime, useUTC)
				}
			}
		}

		// Create a new printer item
		item := map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      p.GetName(),
				"namespace": p.GetNamespace(),
			},
			"source":   source,
			"target":   target,
			"created":  output.FormatTimestamp(creationTime.Time, useUTC),
			"vms":      vmStatus,
			"ready":    fmt.Sprintf("%t", planDetails.IsReady),
			"running":  fmt.Sprintf("%t", planDetails.RunningMigration != nil),
			"status":   planDetails.Status,
			"progress": progressStatus,
			"cutover":  cutoverInfo,
			"archived": fmt.Sprintf("%t", archived),
			"object":   p.Object, // Include the original object
		}

		// Add the item to the list
		items = append(items, item)
	}

	// Apply query filter
	if query != "" {
		queryOpts, err := querypkg.ParseQueryString(query)
		if err != nil {
			return fmt.Errorf("failed to parse query: %v", err)
		}
		items, err = querypkg.ApplyQuery(items, queryOpts)
		if err != nil {
			return fmt.Errorf("error applying query: %v", err)
		}
	}

	// Handle different output formats
	switch outputFormat {
	case "json":
		// Use JSON printer
		jsonPrinter := output.NewJSONPrinter().
			WithPrettyPrint(true).
			AddItems(items)

		if len(items) == 0 {
			return jsonPrinter.PrintEmpty("No plans found in namespace " + namespace)
		}
		return jsonPrinter.Print()
	case "yaml":
		yamlPrinter := output.NewYAMLPrinter().
			AddItems(items)

		if len(items) == 0 {
			return yamlPrinter.PrintEmpty("No plans found in namespace " + namespace)
		}
		return yamlPrinter.Print()
	}

	var headers []output.Column

	headers = append(headers, output.Column{Title: "NAME", Key: "metadata.name"})

	if namespace == "" {
		headers = append(headers, output.Column{Title: "NAMESPACE", Key: "metadata.namespace"})
	}

	headers = append(headers,
		output.Column{Title: "SOURCE", Key: "source"},
		output.Column{Title: "TARGET", Key: "target"},
		output.Column{Title: "VMS", Key: "vms"},
		output.Column{Title: "READY", Key: "ready", ColorFunc: output.ColorizeConditionStatus},
		output.Column{Title: "STATUS", Key: "status", ColorFunc: output.ColorizeStatus},
		output.Column{Title: "PROGRESS", Key: "progress"},
		output.Column{Title: "CUTOVER", Key: "cutover"},
		output.Column{Title: "ARCHIVED", Key: "archived"},
		output.Column{Title: "CREATED", Key: "created"},
	)

	tablePrinter := output.NewTablePrinter().WithColumns(headers...).AddItems(items)

	emptyMsg := "No plans found in namespace " + namespace
	if outputFormat == "markdown" {
		if len(items) == 0 {
			if err := tablePrinter.PrintEmpty(emptyMsg); err != nil {
				return fmt.Errorf("error printing empty markdown: %v", err)
			}
		} else if err := tablePrinter.PrintMarkdown(); err != nil {
			return fmt.Errorf("error printing markdown: %v", err)
		}
	} else {
		if len(items) == 0 {
			if err := tablePrinter.PrintEmpty(emptyMsg); err != nil {
				return fmt.Errorf("error printing empty table: %v", err)
			}
		} else if err := tablePrinter.Print(); err != nil {
			return fmt.Errorf("error printing table: %v", err)
		}
	}

	return nil
}

// List lists migration plans with optional watch mode
func List(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string, watchMode bool, outputFormat string, planName string, useUTC bool, query string) error {
	return watch.WrapWithWatch(watchMode, outputFormat, func() error {
		return ListPlans(ctx, configFlags, namespace, outputFormat, planName, useUTC, query)
	}, watch.DefaultInterval)
}
