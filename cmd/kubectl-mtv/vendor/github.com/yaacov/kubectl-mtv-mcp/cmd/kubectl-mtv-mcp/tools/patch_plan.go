package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// PatchPlanInput represents the input for PatchPlan
type PatchPlanInput struct {
	PlanName                       string `json:"plan_name" jsonschema:"required"`
	Namespace                      string `json:"namespace,omitempty"`
	TransferNetwork                string `json:"transfer_network,omitempty"`
	InstallLegacyDrivers           string `json:"install_legacy_drivers,omitempty"`
	MigrationType                  string `json:"migration_type,omitempty"`
	TargetLabels                   string `json:"target_labels,omitempty"`
	TargetNodeSelector             string `json:"target_node_selector,omitempty"`
	UseCompatibilityMode           *bool  `json:"use_compatibility_mode,omitempty"`
	TargetAffinity                 string `json:"target_affinity,omitempty"`
	TargetNamespace                string `json:"target_namespace,omitempty"`
	TargetPowerState               string `json:"target_power_state,omitempty"`
	Description                    string `json:"description,omitempty"`
	PreserveClusterCPUModel        *bool  `json:"preserve_cluster_cpu_model,omitempty"`
	PreserveStaticIPs              *bool  `json:"preserve_static_ips,omitempty"`
	PVCNameTemplate                string `json:"pvc_name_template,omitempty"`
	VolumeNameTemplate             string `json:"volume_name_template,omitempty"`
	NetworkNameTemplate            string `json:"network_name_template,omitempty"`
	MigrateSharedDisks             *bool  `json:"migrate_shared_disks,omitempty"`
	Archived                       *bool  `json:"archived,omitempty"`
	PVCNameTemplateUseGenerateName *bool  `json:"pvc_name_template_use_generate_name,omitempty"`
	DeleteGuestConversionPod       *bool  `json:"delete_guest_conversion_pod,omitempty"`
	DeleteVMOnFailMigration        *bool  `json:"delete_vm_on_fail_migration,omitempty"`
	SkipGuestConversion            *bool  `json:"skip_guest_conversion,omitempty"`
	Warm                           *bool  `json:"warm,omitempty"`
	RunPreflightInspection         *bool  `json:"run_preflight_inspection,omitempty"`
	ConvertorLabels                string `json:"convertor_labels,omitempty"`
	ConvertorNodeSelector          string `json:"convertor_node_selector,omitempty"`
	ConvertorAffinity              string `json:"convertor_affinity,omitempty"`
}

// GetPatchPlanTool returns the tool definition
func GetPatchPlanTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "PatchPlan",
		Description: `Patch/modify various fields of an existing migration plan without modifying its VMs.

    This allows updating plan configuration without recreating the entire plan. You can modify
    individual plan properties while preserving the VM list and other unchanged settings.

    Boolean Parameters:
    - None (default): Don't change the current value
    - True: Set to true
    - False: Set to false

    Migration Types:
    - cold: Traditional migration with VM shutdown (most reliable)
    - warm: Warm migration with reduced downtime (initial copy while VM runs)
    - live: Minimal downtime migration (advanced, limited compatibility)
    - conversion: Only perform guest OS conversion without disk transfer

    Target Power State:
    - on: Start VMs after migration
    - off: Leave VMs stopped after migration
    - auto: Match source VM power state

    Legacy Drivers:
    - true: Install legacy Windows drivers
    - false: Don't install legacy drivers
    - (empty): Auto-detect based on guest OS

    Args:
        plan_name: Name of the migration plan to patch (required)
        namespace: Kubernetes namespace containing the plan (optional)
        transfer_network: Network to use for transferring VM data - supports 'namespace/network-name' or just 'network-name' (uses plan namespace) (optional)
        install_legacy_drivers: Install legacy drivers - 'true', 'false', or empty for auto (optional)
        migration_type: Migration type - 'cold', 'warm', 'live', or 'conversion' (optional)
        target_labels: Target VM labels - 'key=value,key2=value2' format (optional)
        target_node_selector: Target node selector - 'key=value,key2=value2' format (optional)
        use_compatibility_mode: Use compatibility mode for migration (optional)
        target_affinity: Target affinity using KARL syntax (optional)
            KARL (Kubernetes Affinity Rule Language) provides human-readable syntax for pod scheduling rules.

            KARL Rule Syntax: [RULE_TYPE] pods([SELECTORS]) on [TOPOLOGY]
            - Rule Types: REQUIRE, PREFER, AVOID, REPEL
            - Target: Only pods() supported (no node affinity)
            - Topology: node, zone, region, rack
            - No AND/OR logic support (single rule only)

            KARL Examples:
            - 'REQUIRE pods(app=database) on node' - Co-locate with database pods
            - 'PREFER pods(tier=web) on zone' - Prefer same zone as web pods
            - 'AVOID pods(app=cache) on node' - Separate from cache pods on same node
            - 'REPEL pods(workload=heavy) on zone weight=80' - Soft avoid in same zone
        target_namespace: Target namespace for migrated VMs (optional)
        target_power_state: Target power state - 'on', 'off', or 'auto' (optional)
        description: Plan description (optional)
        preserve_cluster_cpu_model: Preserve CPU model from oVirt cluster (optional)
        preserve_static_ips: Preserve static IPs of vSphere VMs (optional)
        pvc_name_template: Template for generating PVC names (optional)
        volume_name_template: Template for generating volume interface names (optional)
        network_name_template: Template for generating network interface names (optional)
        migrate_shared_disks: Whether to migrate shared disks (optional)
        archived: Whether plan should be archived (optional)
        pvc_name_template_use_generate_name: Use generateName for PVC template (optional)
        delete_guest_conversion_pod: Delete conversion pod after migration (optional)
        delete_vm_on_fail_migration: Delete target VM when migration fails (optional)
        skip_guest_conversion: Skip guest conversion process (optional)
        warm: Enable warm migration (optional, prefer migration_type parameter)
        run_preflight_inspection: Run preflight inspection on VM base disks (optional, applies only to warm migrations from VMware)
        convertor_labels: Labels for virt-v2v convertor pods - 'key1=value1,key2=value2' format (optional)
        convertor_node_selector: Node selector for convertor pod scheduling - 'key1=value1,key2=value2' format (optional)
        convertor_affinity: Convertor affinity using KARL syntax - e.g. 'REQUIRE pods(app=storage) on node' (optional)

    Returns:
        Command output confirming plan patch

    Examples:
        # Update migration type and target namespace
        patch_plan(plan_name="my-plan", migration_type="warm", target_namespace="migrated-vms")

        # Enable compatibility mode and set target power state
        patch_plan(plan_name="my-plan", use_compatibility_mode=true, target_power_state="on")

        # Add KARL affinity rules for co-location with database
        patch_plan(plan_name="my-plan", target_affinity="REQUIRE pods(app=database) on node")

        # Pod anti-affinity to spread VMs across different nodes
        patch_plan(plan_name="distributed-app", target_affinity="AVOID pods(app=web) on node")

        # Archive plan and add description
        patch_plan(plan_name="my-plan", archived=true, description="Completed production migration")`,
	}
}

func HandlePatchPlan(ctx context.Context, req *mcp.CallToolRequest, input PatchPlanInput) (*mcp.CallToolResult, any, error) {
	// Validate required parameters
	if err := mtvmcp.ValidateRequiredParams(map[string]string{
		"plan_name": input.PlanName,
	}); err != nil {
		return nil, "", err
	}

	args := []string{"patch", "plan", input.PlanName}

	if input.Namespace != "" {
		args = append(args, "-n", input.Namespace)
	}

	// Add string parameters
	if input.TransferNetwork != "" {
		args = append(args, "--transfer-network", input.TransferNetwork)
	}
	if input.InstallLegacyDrivers != "" {
		args = append(args, "--install-legacy-drivers", input.InstallLegacyDrivers)
	}
	if input.MigrationType != "" {
		mt := strings.ToLower(strings.TrimSpace(input.MigrationType))
		switch mt {
		case "cold", "warm", "live", "conversion":
			args = append(args, "--migration-type", mt)
		default:
			return nil, "", fmt.Errorf("invalid migration_type: %s (valid: cold|warm|live|conversion)", input.MigrationType)
		}
	}
	if input.TargetLabels != "" {
		args = append(args, "--target-labels", input.TargetLabels)
	}
	if input.TargetNodeSelector != "" {
		args = append(args, "--target-node-selector", input.TargetNodeSelector)
	}
	if input.TargetAffinity != "" {
		args = append(args, "--target-affinity", input.TargetAffinity)
	}
	if input.TargetNamespace != "" {
		args = append(args, "--target-namespace", input.TargetNamespace)
	}
	if input.TargetPowerState != "" {
		ps := strings.ToLower(strings.TrimSpace(input.TargetPowerState))
		switch ps {
		case "on", "off", "auto":
			args = append(args, "--target-power-state", ps)
		default:
			return nil, "", fmt.Errorf("invalid target_power_state: %s (valid: on|off|auto)", input.TargetPowerState)
		}
	}
	if input.Description != "" {
		args = append(args, "--description", input.Description)
	}
	if input.PVCNameTemplate != "" {
		args = append(args, "--pvc-name-template", input.PVCNameTemplate)
	}
	if input.VolumeNameTemplate != "" {
		args = append(args, "--volume-name-template", input.VolumeNameTemplate)
	}
	if input.NetworkNameTemplate != "" {
		args = append(args, "--network-name-template", input.NetworkNameTemplate)
	}

	// Add boolean parameters
	mtvmcp.AddBooleanFlag(&args, "use-compatibility-mode", input.UseCompatibilityMode)
	mtvmcp.AddBooleanFlag(&args, "preserve-cluster-cpu-model", input.PreserveClusterCPUModel)
	mtvmcp.AddBooleanFlag(&args, "preserve-static-ips", input.PreserveStaticIPs)
	mtvmcp.AddBooleanFlag(&args, "migrate-shared-disks", input.MigrateSharedDisks)
	mtvmcp.AddBooleanFlag(&args, "archived", input.Archived)
	mtvmcp.AddBooleanFlag(&args, "pvc-name-template-use-generate-name", input.PVCNameTemplateUseGenerateName)
	mtvmcp.AddBooleanFlag(&args, "delete-guest-conversion-pod", input.DeleteGuestConversionPod)
	mtvmcp.AddBooleanFlag(&args, "delete-vm-on-fail-migration", input.DeleteVMOnFailMigration)
	mtvmcp.AddBooleanFlag(&args, "skip-guest-conversion", input.SkipGuestConversion)
	if input.MigrationType == "" {
		mtvmcp.AddBooleanFlag(&args, "warm", input.Warm)
	}

	// Add new preflight and convertor flags
	mtvmcp.AddBooleanFlag(&args, "run-preflight-inspection", input.RunPreflightInspection)
	if input.ConvertorLabels != "" {
		args = append(args, "--convertor-labels", input.ConvertorLabels)
	}
	if input.ConvertorNodeSelector != "" {
		args = append(args, "--convertor-node-selector", input.ConvertorNodeSelector)
	}
	if input.ConvertorAffinity != "" {
		args = append(args, "--convertor-affinity", input.ConvertorAffinity)
	}

	result, err := mtvmcp.RunKubectlMTVCommand(args)
	if err != nil {
		return nil, "", err
	}

	// Unmarshal the full CommandResponse to provide complete diagnostic information
	data, err := mtvmcp.UnmarshalJSONResponse(result)
	if err != nil {
		return nil, "", err
	}
	return nil, data, nil
}
