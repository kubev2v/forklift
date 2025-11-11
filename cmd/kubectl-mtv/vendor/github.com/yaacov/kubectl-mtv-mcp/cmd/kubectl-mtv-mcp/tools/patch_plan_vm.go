package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// PatchPlanVmInput represents the input for PatchPlanVm
type PatchPlanVmInput struct {
	PlanName                string `json:"plan_name" jsonschema:"required"`
	VmName                  string `json:"vm_name" jsonschema:"required"`
	Namespace               string `json:"namespace,omitempty"`
	TargetName              string `json:"target_name,omitempty"`
	RootDisk                string `json:"root_disk,omitempty"`
	InstanceType            string `json:"instance_type,omitempty"`
	PVCNameTemplate         string `json:"pvc_name_template,omitempty"`
	VolumeNameTemplate      string `json:"volume_name_template,omitempty"`
	NetworkNameTemplate     string `json:"network_name_template,omitempty"`
	LUKSSecret              string `json:"luks_secret,omitempty"`
	TargetPowerState        string `json:"target_power_state,omitempty"`
	AddPreHook              string `json:"add_pre_hook,omitempty"`
	AddPostHook             string `json:"add_post_hook,omitempty"`
	RemoveHook              string `json:"remove_hook,omitempty"`
	ClearHooks              bool   `json:"clear_hooks,omitempty"`
	DeleteVMOnFailMigration *bool  `json:"delete_vm_on_fail_migration,omitempty"`
	DryRun                  bool   `json:"dry_run,omitempty"`
}

// GetPatchPlanVmTool returns the tool definition
func GetPatchPlanVmTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "PatchPlanVm",
		Description: `Patch VM-specific fields for a VM within a migration plan's VM list.

    This allows you to customize individual VM settings within a plan without affecting other VMs.
    Useful for setting VM-specific configurations like custom names, storage templates, hooks, or LUKS decryption.

    Template Variables:
    Templates support Go template syntax with different variables for each template type:

    PVC Name Template Variables:
    - {{.VmName}} - VM name
    - {{.PlanName}} - Migration plan name
    - {{.DiskIndex}} - Initial volume index of the disk
    - {{.WinDriveLetter}} - Windows drive letter (lowercase, requires guest agent)
    - {{.RootDiskIndex}} - Index of the root disk
    - {{.Shared}} - True if volume is shared by multiple VMs
    - {{.FileName}} - Source file name (vSphere only, requires guest agent)

    Volume Name Template Variables:
    - {{.PVCName}} - Name of the PVC mounted to the VM
    - {{.VolumeIndex}} - Sequential index of volume interface (0-based)

    Network Name Template Variables:
    - {{.NetworkName}} - Multus network attachment definition name (if applicable)
    - {{.NetworkNamespace}} - Namespace of network attachment definition (if applicable)
    - {{.NetworkType}} - Network type ("Multus" or "Pod")
    - {{.NetworkIndex}} - Sequential index of network interface (0-based)

    Available Template Functions:
    Templates support Go text template syntax including the following built-in functions:

    String Functions:
    - lower: Converts string to lowercase → {{ lower "TEXT" }} → text
    - upper: Converts string to uppercase → {{ upper "text" }} → TEXT
    - contains: Checks if string contains substring → {{ contains "hello" "lo" }} → true
    - replace: Replaces occurrences in a string → {{"I Am Henry VIII" | replace " " "-"}} → I-Am-Henry-VIII
    - trim: Removes whitespace from both ends → {{ trim "  text  " }} → text
    - trimAll: Removes specified characters from both ends → {{ trimAll "$" "$5.00$" }} → 5.00
    - trimSuffix: Removes suffix if present → {{ trimSuffix ".go" "file.go" }} → file
    - trimPrefix: Removes prefix if present → {{ trimPrefix "go." "go.file" }} → file
    - title: Converts to title case → {{ title "hello world" }} → Hello World
    - untitle: Converts to lowercase → {{ untitle "Hello World" }} → hello world
    - repeat: Repeats string n times → {{ repeat 3 "abc" }} → abcabcabc
    - substr: Extracts substring from start to end → {{ substr 1 4 "abcdef" }} → bcd
    - nospace: Removes all whitespace → {{ nospace "a b  c" }} → abc
    - trunc: Truncates string to specified length → {{ trunc 3 "abcdef" }} → abc
    - initials: Extracts first letter of each word → {{ initials "John Doe" }} → JD
    - hasPrefix: Checks if string starts with prefix → {{ hasPrefix "go" "golang" }} → true
    - hasSuffix: Checks if string ends with suffix → {{ hasSuffix "ing" "coding" }} → true
    - mustRegexReplaceAll: Replaces matches using regex → {{ mustRegexReplaceAll "a(x*)b" "-ab-axxb-" "${1}W" }} → -W-xxW-

    Math Functions:
    - add: Sum numbers → {{ add 1 2 3 }} → 6
    - add1: Increment by 1 → {{ add1 5 }} → 6
    - sub: Subtract second number from first → {{ sub 5 3 }} → 2
    - div: Integer division → {{ div 10 3 }} → 3
    - mod: Modulo operation → {{ mod 10 3 }} → 1
    - mul: Multiply numbers → {{ mul 2 3 4 }} → 24
    - max: Return largest integer → {{ max 1 5 3 }} → 5
    - min: Return smallest integer → {{ min 1 5 3 }} → 1
    - floor: Round down to nearest integer → {{ floor 3.75 }} → 3.0
    - ceil: Round up to nearest integer → {{ ceil 3.25 }} → 4.0
    - round: Round to specified decimal places → {{ round 3.75159 2 }} → 3.75

    LUKS Secret Usage:
    The luks_secret parameter should reference a Kubernetes Secret containing the actual
    LUKS decryption keys. MTV will use this Secret to decrypt encrypted VM disks during migration.
    The Secret must exist in the same namespace as the migration plan.

    Hook Management:
    - add_pre_hook: Add a pre-migration hook to this VM
    - add_post_hook: Add a post-migration hook to this VM
    - remove_hook: Remove a specific hook by name
    - clear_hooks: Remove all hooks from this VM

    Args:
        plan_name: Name of the migration plan containing the VM (required)
        vm_name: Name of the VM to patch within the plan (required)
        namespace: Kubernetes namespace containing the plan (optional)
        target_name: Custom name for the VM in the target cluster (optional)
        root_disk: The primary disk to boot from (optional)
        instance_type: Override VM's instance type in target (optional)
        pvc_name_template: Go template for naming PVCs for this VM's disks (optional)
        volume_name_template: Go template for naming volume interfaces (optional)
        network_name_template: Go template for naming network interfaces (optional)
        luks_secret: Kubernetes Secret name containing LUKS disk decryption keys (optional)
        target_power_state: Target power state for this VM - 'on', 'off', or 'auto' (optional)
        add_pre_hook: Add a pre-migration hook to this VM (optional)
        add_post_hook: Add a post-migration hook to this VM (optional)
        remove_hook: Remove a hook from this VM by hook name (optional)
        clear_hooks: Remove all hooks from this VM (optional, default False)

    Returns:
        Command output confirming VM patch

    Examples:
        # Customize VM name and power state
        patch_plan_vm("my-plan", "source-vm", target_name="migrated-vm", target_power_state="on")

        # Add hooks to specific VM
        patch_plan_vm("my-plan", "database-vm", add_pre_hook="db-backup", add_post_hook="db-validate")

        # Use custom PVC naming template
        patch_plan_vm("my-plan", "storage-vm", pvc_name_template="{{.VmName}}-disk-{{.DiskIndex}}")`,
	}
}

func HandlePatchPlanVm(ctx context.Context, req *mcp.CallToolRequest, input PatchPlanVmInput) (*mcp.CallToolResult, any, error) {
	// Enable dry run mode if requested
	if input.DryRun {
		ctx = mtvmcp.WithDryRun(ctx, true)
	}

	// Validate required parameters
	if err := mtvmcp.ValidateRequiredParams(map[string]string{
		"plan_name": input.PlanName,
		"vm_name":   input.VmName,
	}); err != nil {
		return nil, "", err
	}

	args := []string{"patch", "planvm", input.PlanName, input.VmName}

	if input.Namespace != "" {
		args = append(args, "-n", input.Namespace)
	}

	// Add VM-specific parameters
	if input.TargetName != "" {
		args = append(args, "--target-name", input.TargetName)
	}
	if input.RootDisk != "" {
		args = append(args, "--root-disk", input.RootDisk)
	}
	if input.InstanceType != "" {
		args = append(args, "--instance-type", input.InstanceType)
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
	if input.LUKSSecret != "" {
		args = append(args, "--luks-secret", input.LUKSSecret)
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

	// VM-level options
	mtvmcp.AddBooleanFlag(&args, "delete-vm-on-fail-migration", input.DeleteVMOnFailMigration)

	// Hook management
	if input.AddPreHook != "" {
		args = append(args, "--add-pre-hook", input.AddPreHook)
	}
	if input.AddPostHook != "" {
		args = append(args, "--add-post-hook", input.AddPostHook)
	}
	if input.RemoveHook != "" {
		args = append(args, "--remove-hook", input.RemoveHook)
	}
	if input.ClearHooks {
		args = append(args, "--clear-hooks")
	}

	result, err := mtvmcp.RunKubectlMTVCommand(ctx, args)
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
