package health

import (
	"context"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// Known FQIN fields in ForkliftController spec
var knownFQINFields = []string{
	"controller_image_fqin",
	"api_image_fqin",
	"validation_image_fqin",
	"ui_plugin_image_fqin",
	"must_gather_image_fqin",
	"virt_v2v_image_fqin",
	"cli_download_image_fqin",
	"populator_controller_image_fqin",
	"populator_ovirt_image_fqin",
	"populator_openstack_image_fqin",
	"populator_vsphere_xcopy_volume_image_fqin",
	"ova_provider_server_fqin",
	"ova_proxy_fqin",
	"hyperv_provider_server_fqin",
}

// CheckControllerHealth checks the health of the ForkliftController CR.
//
// IMPORTANT: The ForkliftController custom resource ALWAYS exists in the operator
// namespace. The caller (health.go) should pass the auto-detected operator
// namespace here, NOT a user-specified namespace.
func CheckControllerHealth(ctx context.Context, configFlags *genericclioptions.ConfigFlags, operatorNamespace string, hasVSphereProvider bool, hasRemoteOpenShiftProvider bool) (ControllerHealth, error) {
	health := ControllerHealth{
		Found:                      false,
		HasVSphereProvider:         hasVSphereProvider,
		HasRemoteOpenShiftProvider: hasRemoteOpenShiftProvider,
		CustomImages:               []ImageOverride{},
	}

	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		health.Error = err.Error()
		return health, err
	}

	// Use provided operator namespace or fall back to default
	ns := operatorNamespace
	if ns == "" {
		ns = client.OpenShiftMTVNamespace
	}

	// List ForkliftControllers in the namespace
	controllers, err := dynamicClient.Resource(client.ForkliftControllersGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		health.Error = err.Error()
		return health, err
	}

	if len(controllers.Items) == 0 {
		return health, nil
	}

	// Use the first controller (typically there's only one)
	controller := &controllers.Items[0]
	health.Found = true
	health.Name = controller.GetName()
	health.Namespace = controller.GetNamespace()

	// Extract spec fields
	extractControllerSpec(controller, &health)

	// Extract status conditions
	extractControllerStatus(controller, &health)

	return health, nil
}

// extractControllerSpec extracts spec fields from ForkliftController
func extractControllerSpec(controller *unstructured.Unstructured, health *ControllerHealth) {
	spec, found, err := unstructured.NestedMap(controller.Object, "spec")
	if err != nil {
		// Spec exists but is malformed - could log this in verbose mode
		return
	}
	if !found {
		return
	}

	// Extract feature flags
	health.FeatureFlags = extractFeatureFlags(spec)

	// Extract VDDK image
	if vddkImage, ok := spec["vddk_image"].(string); ok && vddkImage != "" {
		health.VDDKImage = vddkImage
	}

	// Extract controller log level
	if logLevel, found, err := unstructured.NestedInt64(spec, "controller_log_level"); err == nil && found {
		health.LogLevel = int(logLevel)
	} else if logLevelStr, ok := spec["controller_log_level"].(string); ok {
		if level, err := strconv.Atoi(logLevelStr); err == nil {
			health.LogLevel = level
		}
	}

	// Extract custom FQIN images
	for _, field := range knownFQINFields {
		if image, ok := spec[field].(string); ok && image != "" {
			health.CustomImages = append(health.CustomImages, ImageOverride{
				Field: field,
				Image: image,
			})
		}
	}
}

// extractFeatureFlags extracts feature flag settings from spec
func extractFeatureFlags(spec map[string]interface{}) FeatureFlags {
	flags := FeatureFlags{}

	// Helper function to parse boolean from string or bool
	parseBool := func(value interface{}) *bool {
		if value == nil {
			return nil
		}
		var result bool
		switch v := value.(type) {
		case bool:
			result = v
			return &result
		case string:
			if v == "true" {
				result = true
				return &result
			} else if v == "false" {
				result = false
				return &result
			}
		}
		return nil
	}

	flags.UIPlugin = parseBool(spec["feature_ui_plugin"])
	flags.Validation = parseBool(spec["feature_validation"])
	flags.VolumePopulator = parseBool(spec["feature_volume_populator"])
	flags.AuthRequired = parseBool(spec["feature_auth_required"])
	flags.OCPLiveMigration = parseBool(spec["feature_ocp_live_migration"])

	return flags
}

// extractControllerStatus extracts status conditions from ForkliftController
func extractControllerStatus(controller *unstructured.Unstructured, health *ControllerHealth) {
	conditions, found, _ := unstructured.NestedSlice(controller.Object, "status", "conditions")
	if !found {
		return
	}

	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _, _ := unstructured.NestedString(condition, "type")
		condStatus, _, _ := unstructured.NestedString(condition, "status")
		message, _, _ := unstructured.NestedString(condition, "message")

		switch condType {
		case "Running":
			health.Status.Running = condStatus == "True"
		case "Successful":
			health.Status.Successful = condStatus == "True"
		case "Failure":
			health.Status.Failed = condStatus == "True"
			if condStatus == "True" && message != "" {
				health.Status.Message = message
			}
		}
	}
}

// AnalyzeControllerHealth analyzes the controller health and returns issues
func AnalyzeControllerHealth(health *ControllerHealth, report *HealthReport) {
	// If there was an API error, don't report "not found" since we couldn't actually check
	if health.Error != "" {
		// Error was already reported by the caller, skip further analysis
		return
	}

	if !health.Found {
		report.AddIssue(
			SeverityCritical,
			"Controller",
			"ForkliftController",
			"ForkliftController not found",
			"Create a ForkliftController resource in the MTV namespace",
		)
		return
	}

	// Check for vSphere providers without VDDK image
	if health.HasVSphereProvider && health.VDDKImage == "" {
		report.AddIssue(
			SeverityCritical,
			"Controller",
			health.Name,
			"vSphere providers detected but vddk_image not configured - VMware migrations will fail",
			"Set vddk_image in ForkliftController spec or run: kubectl mtv create vddk-image --help",
		)
	}

	// Check for controller failure
	if health.Status.Failed {
		message := "ForkliftController is in Failed state"
		if health.Status.Message != "" {
			message += ": " + health.Status.Message
		}
		report.AddIssue(
			SeverityCritical,
			"Controller",
			health.Name,
			message,
			"Check the forklift-operator logs for details",
		)
	}

	// Check for custom images (informational)
	if len(health.CustomImages) > 0 {
		report.AddIssue(
			SeverityInfo,
			"Controller",
			health.Name,
			"Custom container images configured - ensure registries are accessible",
			"Verify custom images are available: "+health.CustomImages[0].Image,
		)
	}

	// Check for high log level (informational)
	if health.LogLevel > 3 {
		report.AddIssue(
			SeverityInfo,
			"Controller",
			health.Name,
			"High controller log level configured ("+strconv.Itoa(health.LogLevel)+") - may impact performance",
			"Consider reducing controller_log_level for production use",
		)
	}

	// Check for remote OpenShift provider without OCP live migration enabled
	if health.HasRemoteOpenShiftProvider {
		liveMigrationEnabled := health.FeatureFlags.OCPLiveMigration != nil && *health.FeatureFlags.OCPLiveMigration
		if !liveMigrationEnabled {
			report.AddIssue(
				SeverityWarning,
				"Controller",
				health.Name,
				"Remote OpenShift provider detected but feature_ocp_live_migration not enabled - live migrations will not be available",
				"Set feature_ocp_live_migration: true in ForkliftController spec to enable cross-cluster live migration",
			)
		}
	}
}
