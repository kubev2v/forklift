// Package settings provides functionality for managing ForkliftController settings.
package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// GetSettingsOptions contains options for getting settings.
type GetSettingsOptions struct {
	ConfigFlags *genericclioptions.ConfigFlags
	SettingName string // optional: get a specific setting
	AllSettings bool   // if true, return all settings (supported + extended)
}

// SetSettingOptions contains options for setting a value.
type SetSettingOptions struct {
	ConfigFlags *genericclioptions.ConfigFlags
	Name        string
	Value       string
	Verbosity   int
}

// UnsetSettingOptions contains options for unsetting a value.
type UnsetSettingOptions struct {
	ConfigFlags *genericclioptions.ConfigFlags
	Name        string
	Verbosity   int
}

// wrapClusterError wraps cluster connection errors with user-friendly messages.
func wrapClusterError(err error, operation string) error {
	errStr := err.Error()

	// Connection refused - cluster not accessible
	if strings.Contains(errStr, "connection refused") {
		return fmt.Errorf("cannot connect to Kubernetes cluster: %w\n\nMake sure:\n  - Your kubeconfig is correctly configured\n  - The cluster is running and accessible\n  - Run 'kubectl cluster-info' to verify connectivity", err)
	}

	// Unauthorized - authentication issues
	if strings.Contains(errStr, "Unauthorized") || strings.Contains(errStr, "unauthorized") {
		return fmt.Errorf("authentication failed: %w\n\nMake sure:\n  - Your kubeconfig credentials are valid\n  - Run 'kubectl auth can-i get forkliftcontrollers -n openshift-mtv' to check permissions", err)
	}

	// Forbidden - permission issues
	if strings.Contains(errStr, "forbidden") || strings.Contains(errStr, "Forbidden") {
		return fmt.Errorf("permission denied: %w\n\nMake sure you have permissions to access ForkliftController resources", err)
	}

	// Resource not found - MTV not installed
	if strings.Contains(errStr, "the server could not find the requested resource") ||
		strings.Contains(errStr, "no matches for kind") {
		return fmt.Errorf("MTV (Migration Toolkit for Virtualization) is not installed on this cluster\n\nThe ForkliftController CRD was not found. Please install MTV first.")
	}

	// Default: return original error with operation context
	return fmt.Errorf("%s: %w", operation, err)
}

// GetSettings retrieves the current ForkliftController settings.
// If settingName is empty, settings are returned based on AllSettings flag.
// If settingName is specified, only that setting is returned.
// If AllSettings is true, all settings (supported + extended) are returned.
func GetSettings(ctx context.Context, opts GetSettingsOptions) ([]SettingValue, error) {
	// Get the MTV operator namespace
	operatorNamespace := client.GetMTVOperatorNamespace(ctx, opts.ConfigFlags)

	// Get dynamic client
	dynamicClient, err := client.GetDynamicClient(opts.ConfigFlags)
	if err != nil {
		return nil, wrapClusterError(err, "failed to create Kubernetes client")
	}

	// List ForkliftController resources in the operator namespace
	controllerList, err := dynamicClient.Resource(client.ForkliftControllersGVR).Namespace(operatorNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, wrapClusterError(err, "failed to access ForkliftController")
	}

	if len(controllerList.Items) == 0 {
		return nil, fmt.Errorf("no ForkliftController found in namespace '%s'\n\nMake sure MTV is properly installed and the ForkliftController CR exists", operatorNamespace)
	}

	// Use the first ForkliftController (typically there's only one)
	controller := &controllerList.Items[0]

	// Extract spec
	spec, _, err := unstructured.NestedMap(controller.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("failed to get ForkliftController spec: %w", err)
	}

	// Determine which settings map to use
	settingsMap := SupportedSettings
	if opts.AllSettings {
		settingsMap = GetAllSettings()
	}

	// Build the result
	var result []SettingValue

	// If a specific setting is requested, only return that one
	if opts.SettingName != "" {
		// For specific setting lookups, check all settings (not just the filtered set)
		allSettings := GetAllSettings()
		def, ok := allSettings[opts.SettingName]
		if !ok {
			return nil, fmt.Errorf("unknown setting: %s\nUse 'kubectl mtv settings --all' to see all available settings", opts.SettingName)
		}
		sv := extractSettingValue(spec, def)
		return []SettingValue{sv}, nil
	}

	// Return all settings in category order with deterministic ordering within each category
	for _, category := range CategoryOrder {
		// Collect names for this category
		var categoryNames []string
		for name, def := range settingsMap {
			if def.Category == category {
				categoryNames = append(categoryNames, name)
			}
		}
		// Sort names within category for deterministic ordering
		sort.Strings(categoryNames)
		// Iterate sorted names and build result
		for _, name := range categoryNames {
			def := settingsMap[name]
			sv := extractSettingValue(spec, def)
			sv.Name = name
			result = append(result, sv)
		}
	}

	return result, nil
}

// extractSettingValue extracts a setting value from the ForkliftController spec.
func extractSettingValue(spec map[string]interface{}, def SettingDefinition) SettingValue {
	sv := SettingValue{
		Name:       def.Name,
		Default:    def.Default,
		Definition: def,
		IsSet:      false,
	}

	if spec == nil {
		return sv
	}

	rawValue, exists := spec[def.Name]
	if !exists {
		return sv
	}

	sv.IsSet = true

	// Convert the value based on type
	switch def.Type {
	case TypeBool:
		switch v := rawValue.(type) {
		case bool:
			sv.Value = v
		case string:
			if v == "true" {
				sv.Value = true
			} else if v == "false" {
				sv.Value = false
			}
		}
	case TypeInt:
		switch v := rawValue.(type) {
		case int64:
			sv.Value = int(v)
		case float64:
			sv.Value = int(v)
		case int:
			sv.Value = v
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				sv.Value = i
			}
		}
	case TypeString:
		if v, ok := rawValue.(string); ok {
			sv.Value = v
		}
	}

	return sv
}

// SetSetting updates a ForkliftController setting.
func SetSetting(ctx context.Context, opts SetSettingOptions) error {
	// Validate setting name against all known settings
	allSettings := GetAllSettings()
	def, ok := allSettings[opts.Name]
	if !ok {
		return fmt.Errorf("unknown setting: %s\nUse 'kubectl mtv settings --all' to see available settings", opts.Name)
	}

	// Validate and convert the value
	patchValue, err := validateAndConvertValue(opts.Value, def)
	if err != nil {
		return fmt.Errorf("invalid value for %s: %w", opts.Name, err)
	}

	// Get the MTV operator namespace
	operatorNamespace := client.GetMTVOperatorNamespace(ctx, opts.ConfigFlags)
	if opts.Verbosity > 0 {
		fmt.Printf("Using MTV operator namespace: %s\n", operatorNamespace)
	}

	// Get dynamic client
	dynamicClient, err := client.GetDynamicClient(opts.ConfigFlags)
	if err != nil {
		return wrapClusterError(err, "failed to create Kubernetes client")
	}

	// List ForkliftController resources in the operator namespace
	controllerList, err := dynamicClient.Resource(client.ForkliftControllersGVR).Namespace(operatorNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return wrapClusterError(err, "failed to access ForkliftController")
	}

	if len(controllerList.Items) == 0 {
		return fmt.Errorf("no ForkliftController found in namespace '%s'\n\nMake sure MTV is properly installed and the ForkliftController CR exists", operatorNamespace)
	}

	// Use the first ForkliftController (typically there's only one)
	controller := controllerList.Items[0]
	controllerName := controller.GetName()

	// Create the patch data
	patchMap := map[string]interface{}{
		"spec": map[string]interface{}{
			opts.Name: patchValue,
		},
	}

	patchData, err := json.Marshal(patchMap)
	if err != nil {
		return fmt.Errorf("failed to create patch: %w", err)
	}

	if opts.Verbosity > 0 {
		fmt.Printf("Patching ForkliftController '%s': %s\n", controllerName, string(patchData))
	}

	// Apply the patch
	_, err = dynamicClient.Resource(client.ForkliftControllersGVR).Namespace(operatorNamespace).Patch(
		ctx,
		controllerName,
		types.MergePatchType,
		patchData,
		metav1.PatchOptions{},
	)
	if err != nil {
		return wrapClusterError(err, "failed to update setting")
	}

	return nil
}

// UnsetSetting removes a ForkliftController setting (reverts to default).
func UnsetSetting(ctx context.Context, opts UnsetSettingOptions) error {
	// Validate setting name against all known settings
	allSettings := GetAllSettings()
	_, ok := allSettings[opts.Name]
	if !ok {
		return fmt.Errorf("unknown setting: %s\nUse 'kubectl mtv settings --all' to see available settings", opts.Name)
	}

	// Get the MTV operator namespace
	operatorNamespace := client.GetMTVOperatorNamespace(ctx, opts.ConfigFlags)
	if opts.Verbosity > 0 {
		fmt.Printf("Using MTV operator namespace: %s\n", operatorNamespace)
	}

	// Get dynamic client
	dynamicClient, err := client.GetDynamicClient(opts.ConfigFlags)
	if err != nil {
		return wrapClusterError(err, "failed to create Kubernetes client")
	}

	// List ForkliftController resources in the operator namespace
	controllerList, err := dynamicClient.Resource(client.ForkliftControllersGVR).Namespace(operatorNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return wrapClusterError(err, "failed to access ForkliftController")
	}

	if len(controllerList.Items) == 0 {
		return fmt.Errorf("no ForkliftController found in namespace '%s'\n\nMake sure MTV is properly installed and the ForkliftController CR exists", operatorNamespace)
	}

	// Use the first ForkliftController (typically there's only one)
	controller := controllerList.Items[0]
	controllerName := controller.GetName()

	// Create the patch data to remove the field (set to null in JSON merge patch)
	// Using a raw JSON string to properly set null value
	patchData := []byte(fmt.Sprintf(`{"spec":{"%s":null}}`, opts.Name))

	if opts.Verbosity > 0 {
		fmt.Printf("Patching ForkliftController '%s': %s\n", controllerName, string(patchData))
	}

	// Apply the patch
	_, err = dynamicClient.Resource(client.ForkliftControllersGVR).Namespace(operatorNamespace).Patch(
		ctx,
		controllerName,
		types.MergePatchType,
		patchData,
		metav1.PatchOptions{},
	)
	if err != nil {
		return wrapClusterError(err, "failed to remove setting")
	}

	return nil
}

// validateAndConvertValue validates and converts a string value to the appropriate type.
func validateAndConvertValue(value string, def SettingDefinition) (interface{}, error) {
	switch def.Type {
	case TypeBool:
		switch value {
		case "true", "True", "TRUE", "1", "yes", "Yes", "YES":
			return true, nil
		case "false", "False", "FALSE", "0", "no", "No", "NO":
			return false, nil
		default:
			return nil, fmt.Errorf("expected boolean value (true/false), got: %s", value)
		}
	case TypeInt:
		i, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("expected integer value, got: %s", value)
		}
		return i, nil
	case TypeString:
		return value, nil
	default:
		return value, nil
	}
}

// FormatValue formats a setting value for display.
func FormatValue(sv SettingValue) string {
	if !sv.IsSet || sv.Value == nil {
		return "(not set)"
	}

	switch v := sv.Value.(type) {
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case string:
		if v == "" {
			return "(empty)"
		}
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// FormatDefault formats a default value for display.
func FormatDefault(def SettingDefinition) string {
	switch v := def.Default.(type) {
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case string:
		if v == "" {
			return "-"
		}
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
