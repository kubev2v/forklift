package patch

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/patch/plan"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewPlanCmd creates the patch plan command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	// Editable PlanSpec fields
	var transferNetwork string
	var installLegacyDrivers string // "true", "false", or "" for nil
	migrationTypeFlag := flags.NewMigrationTypeFlag()
	var targetLabels []string
	var targetNodeSelector []string
	var useCompatibilityMode bool
	var targetAffinity string
	var targetNamespace string
	var targetPowerState string

	// Convertor-related flags
	var convertorLabels []string
	var convertorNodeSelector []string
	var convertorAffinity string

	// Missing flags from create plan
	var description string
	var preserveClusterCPUModel bool
	var preserveStaticIPs bool
	var pvcNameTemplate string
	var volumeNameTemplate string
	var networkNameTemplate string
	var migrateSharedDisks bool
	var archived bool
	var pvcNameTemplateUseGenerateName bool
	var deleteGuestConversionPod bool
	var deleteVmOnFailMigration bool
	var skipGuestConversion bool
	var warm bool
	var runPreflightInspection bool

	// Boolean tracking for flag changes
	var useCompatibilityModeChanged bool
	var preserveClusterCPUModelChanged bool
	var preserveStaticIPsChanged bool
	var migrateSharedDisksChanged bool
	var archivedChanged bool
	var pvcNameTemplateUseGenerateNameChanged bool
	var deleteGuestConversionPodChanged bool
	var deleteVmOnFailMigrationChanged bool
	var skipGuestConversionChanged bool
	var warmChanged bool
	var runPreflightInspectionChanged bool

	cmd := &cobra.Command{
		Use:   "plan PLAN_NAME",
		Short: "Patch a migration plan",
		Long: `Patch an existing migration plan without modifying its VM list.

Use this to update plan settings like migration type, transfer network,
target labels, node selectors, or convertor pod configuration.`,
		Example: `  # Change migration type to warm
  kubectl-mtv patch plan my-migration --migration-type warm

  # Update transfer network
  kubectl-mtv patch plan my-migration --transfer-network my-namespace/migration-net

  # Add target labels to migrated VMs
  kubectl-mtv patch plan my-migration --target-labels env=prod,team=platform

  # Configure convertor pod scheduling
  kubectl-mtv patch plan my-migration --convertor-node-selector node-role=worker`,
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.PlanNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get plan name from positional argument
			planName := args[0]

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			// Check if boolean flags have been explicitly set (changed from default)
			useCompatibilityModeChanged = cmd.Flags().Changed("use-compatibility-mode")
			preserveClusterCPUModelChanged = cmd.Flags().Changed("preserve-cluster-cpu-model")
			preserveStaticIPsChanged = cmd.Flags().Changed("preserve-static-ips")
			migrateSharedDisksChanged = cmd.Flags().Changed("migrate-shared-disks")
			archivedChanged = cmd.Flags().Changed("archived")
			pvcNameTemplateUseGenerateNameChanged = cmd.Flags().Changed("pvc-name-template-use-generate-name")
			deleteGuestConversionPodChanged = cmd.Flags().Changed("delete-guest-conversion-pod")
			deleteVmOnFailMigrationChanged = cmd.Flags().Changed("delete-vm-on-fail-migration")
			skipGuestConversionChanged = cmd.Flags().Changed("skip-guest-conversion")
			warmChanged = cmd.Flags().Changed("warm")
			runPreflightInspectionChanged = cmd.Flags().Changed("run-preflight-inspection")

			return plan.PatchPlan(plan.PatchPlanOptions{
				ConfigFlags: kubeConfigFlags,
				Name:        planName,
				Namespace:   namespace,

				// Core plan fields
				TransferNetwork:      transferNetwork,
				InstallLegacyDrivers: installLegacyDrivers,
				MigrationType:        string(migrationTypeFlag.GetValue()),
				TargetLabels:         targetLabels,
				TargetNodeSelector:   targetNodeSelector,
				UseCompatibilityMode: useCompatibilityMode,
				TargetAffinity:       targetAffinity,
				TargetNamespace:      targetNamespace,
				TargetPowerState:     targetPowerState,

				// Convertor-related fields
				ConvertorLabels:       convertorLabels,
				ConvertorNodeSelector: convertorNodeSelector,
				ConvertorAffinity:     convertorAffinity,

				// Additional plan fields
				Description:                    description,
				PreserveClusterCPUModel:        preserveClusterCPUModel,
				PreserveStaticIPs:              preserveStaticIPs,
				PVCNameTemplate:                pvcNameTemplate,
				VolumeNameTemplate:             volumeNameTemplate,
				NetworkNameTemplate:            networkNameTemplate,
				MigrateSharedDisks:             migrateSharedDisks,
				Archived:                       archived,
				PVCNameTemplateUseGenerateName: pvcNameTemplateUseGenerateName,
				DeleteGuestConversionPod:       deleteGuestConversionPod,
				SkipGuestConversion:            skipGuestConversion,
				Warm:                           warm,
				RunPreflightInspection:         runPreflightInspection,

				// Flag change tracking
				UseCompatibilityModeChanged:           useCompatibilityModeChanged,
				PreserveClusterCPUModelChanged:        preserveClusterCPUModelChanged,
				PreserveStaticIPsChanged:              preserveStaticIPsChanged,
				MigrateSharedDisksChanged:             migrateSharedDisksChanged,
				ArchivedChanged:                       archivedChanged,
				PVCNameTemplateUseGenerateNameChanged: pvcNameTemplateUseGenerateNameChanged,
				DeleteGuestConversionPodChanged:       deleteGuestConversionPodChanged,
				DeleteVmOnFailMigration:               deleteVmOnFailMigration,
				DeleteVmOnFailMigrationChanged:        deleteVmOnFailMigrationChanged,
				SkipGuestConversionChanged:            skipGuestConversionChanged,
				WarmChanged:                           warmChanged,
				RunPreflightInspectionChanged:         runPreflightInspectionChanged,
			})
		},
	}

	cmd.Flags().StringVar(&transferNetwork, "transfer-network", "", "Network to use for transferring VM data. Supports 'namespace/network-name' or just 'network-name' (uses plan namespace)")
	cmd.Flags().StringVar(&installLegacyDrivers, "install-legacy-drivers", "", "Install legacy Windows drivers (true/false, leave empty for auto-detection) "+flags.ProvidersConversion)
	cmd.Flags().Var(migrationTypeFlag, "migration-type", "Migration type: cold, warm, live, or conversion "+flags.MigrationTypeSupport)
	cmd.Flags().StringSliceVar(&targetLabels, "target-labels", []string{}, "Target VM labels in format key=value (can be specified multiple times)")
	cmd.Flags().StringSliceVar(&targetNodeSelector, "target-node-selector", []string{}, "Target node selector in format key=value (can be specified multiple times)")
	cmd.Flags().BoolVar(&useCompatibilityMode, "use-compatibility-mode", false, "Use compatibility devices (SATA bus, E1000E NIC) when skipGuestConversion is true "+flags.ProvidersVSphereEC2)
	cmd.Flags().StringVar(&targetAffinity, "target-affinity", "", "Target affinity using KARL syntax (e.g. 'REQUIRE pods(app=database) on node')")
	cmd.Flags().StringVar(&targetNamespace, "target-namespace", "", "Target namespace for migrated VMs")
	cmd.Flags().StringVar(&targetPowerState, "target-power-state", "", "Target power state for VMs after migration: 'on', 'off', or 'auto' (default: match source VM power state)")

	// Convertor-related flags (only apply to providers requiring guest conversion)
	cmd.Flags().StringSliceVar(&convertorLabels, "convertor-labels", nil, "Labels to be added to virt-v2v convertor pods (e.g., key1=value1,key2=value2) "+flags.ProvidersConversion)
	cmd.Flags().StringSliceVar(&convertorNodeSelector, "convertor-node-selector", nil, "Node selector to constrain convertor pod scheduling (e.g., key1=value1,key2=value2) "+flags.ProvidersConversion)
	cmd.Flags().StringVar(&convertorAffinity, "convertor-affinity", "", "Convertor affinity to constrain convertor pod scheduling using KARL syntax "+flags.ProvidersConversion)

	// Plan metadata and configuration flags
	cmd.Flags().StringVar(&description, "description", "", "Plan description")
	cmd.Flags().BoolVar(&preserveClusterCPUModel, "preserve-cluster-cpu-model", false, "Preserve the CPU model and flags the VM runs with in its cluster "+flags.ProvidersOVirt)
	cmd.Flags().BoolVar(&preserveStaticIPs, "preserve-static-ips", false, "Preserve static IP configurations during migration "+flags.ProvidersVSphere)
	cmd.Flags().StringVar(&pvcNameTemplate, "pvc-name-template", "", "Template for generating PVC names for VM disks. Variables: {{.VmName}}, {{.PlanName}}, {{.DiskIndex}}, {{.WinDriveLetter}}, {{.RootDiskIndex}}, {{.Shared}}, {{.FileName}} "+flags.ProvidersVSphereOpenShift)
	cmd.Flags().StringVar(&volumeNameTemplate, "volume-name-template", "", "Template for generating volume interface names in the target VM. Variables: {{.PVCName}}, {{.VolumeIndex}} "+flags.ProvidersVSphere)
	cmd.Flags().StringVar(&networkNameTemplate, "network-name-template", "", "Template for generating network interface names in the target VM. Variables: {{.NetworkName}}, {{.NetworkNamespace}}, {{.NetworkType}}, {{.NetworkIndex}} "+flags.ProvidersVSphere)
	cmd.Flags().BoolVar(&migrateSharedDisks, "migrate-shared-disks", true, "Migrate disks shared between multiple VMs "+flags.ProvidersVSphereOVirt)
	cmd.Flags().BoolVar(&archived, "archived", false, "Whether this plan should be archived")
	cmd.Flags().BoolVar(&pvcNameTemplateUseGenerateName, "pvc-name-template-use-generate-name", true, "Use generateName instead of name for PVC name template "+flags.ProvidersVSphere)
	cmd.Flags().BoolVar(&deleteGuestConversionPod, "delete-guest-conversion-pod", false, "Delete guest conversion pod after successful migration "+flags.ProvidersConversion)
	cmd.Flags().BoolVar(&deleteVmOnFailMigration, "delete-vm-on-fail-migration", false, "Delete target VM when migration fails")
	cmd.Flags().BoolVar(&skipGuestConversion, "skip-guest-conversion", false, "Skip the guest conversion process (raw disk copy mode) "+flags.ProvidersVSphereEC2)
	cmd.Flags().BoolVar(&warm, "warm", false, "Enable warm migration (use --migration-type=warm instead) "+flags.ProvidersVSphereOVirt)
	cmd.Flags().BoolVar(&runPreflightInspection, "run-preflight-inspection", true, "Run preflight inspection on VM base disks before starting disk transfer "+flags.CombineHints(flags.ProvidersVSphere, flags.MigrationWarm))

	// Add completion for migration type flag
	if err := cmd.RegisterFlagCompletionFunc("migration-type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return migrationTypeFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	// Add completion for install legacy drivers flag
	if err := cmd.RegisterFlagCompletionFunc("install-legacy-drivers", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"true", "false"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	// Add completion for target power state flag
	if err := cmd.RegisterFlagCompletionFunc("target-power-state", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"on", "off", "auto"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}

// NewPlanVMCmd creates the patch planvm command
func NewPlanVMCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	// VM-specific fields that can be patched
	var targetName string
	var rootDisk string
	var instanceType string
	var pvcNameTemplate string
	var volumeNameTemplate string
	var networkNameTemplate string
	var luksSecret string
	var targetPowerState string

	// Hook-related flags
	var addPreHook string
	var addPostHook string
	var removeHook string
	var clearHooks bool

	// Additional VM flags
	var deleteVmOnFailMigration bool
	var deleteVmOnFailMigrationChanged bool

	cmd := &cobra.Command{
		Use:               "planvm PLAN_NAME VM_NAME",
		Short:             "Patch a specific VM within a migration plan",
		Long:              `Patch VM-specific fields for a VM within a migration plan's VM list.`,
		Args:              cobra.ExactArgs(2),
		SilenceUsage:      true,
		ValidArgsFunction: completion.PlanNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get arguments
			planName := args[0]
			vmName := args[1]

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			// Check if boolean flags have been explicitly set (changed from default)
			deleteVmOnFailMigrationChanged = cmd.Flags().Changed("delete-vm-on-fail-migration")

			return plan.PatchPlanVM(kubeConfigFlags, planName, vmName, namespace,
				targetName, rootDisk, instanceType, pvcNameTemplate, volumeNameTemplate, networkNameTemplate, luksSecret, targetPowerState,
				addPreHook, addPostHook, removeHook, clearHooks, deleteVmOnFailMigration, deleteVmOnFailMigrationChanged)
		},
	}

	// VM-specific flags
	cmd.Flags().StringVar(&targetName, "target-name", "", "Custom name for the VM in the target cluster")
	cmd.Flags().StringVar(&rootDisk, "root-disk", "", "The primary disk to boot from")
	cmd.Flags().StringVar(&instanceType, "instance-type", "", "Override the VM's instance type in the target")
	cmd.Flags().StringVar(&pvcNameTemplate, "pvc-name-template", "", "Go template for naming PVCs for this VM's disks. Variables: {{.VmName}}, {{.PlanName}}, {{.DiskIndex}}, {{.WinDriveLetter}}, {{.RootDiskIndex}}, {{.Shared}}, {{.FileName}} "+flags.ProvidersVSphereOpenShift)
	cmd.Flags().StringVar(&volumeNameTemplate, "volume-name-template", "", "Go template for naming volume interfaces. Variables: {{.PVCName}}, {{.VolumeIndex}} "+flags.ProvidersVSphere)
	cmd.Flags().StringVar(&networkNameTemplate, "network-name-template", "", "Go template for naming network interfaces. Variables: {{.NetworkName}}, {{.NetworkNamespace}}, {{.NetworkType}}, {{.NetworkIndex}} "+flags.ProvidersVSphere)
	cmd.Flags().StringVar(&luksSecret, "luks-secret", "", "Kubernetes Secret name containing LUKS disk decryption keys "+flags.ProvidersVSphereOVirt)
	cmd.Flags().StringVar(&targetPowerState, "target-power-state", "", "Target power state for this VM after migration: 'on', 'off', or 'auto' (default: match source VM power state)")

	// Hook-related flags
	cmd.Flags().StringVar(&addPreHook, "add-pre-hook", "", "Add a pre-migration hook to this VM")
	cmd.Flags().StringVar(&addPostHook, "add-post-hook", "", "Add a post-migration hook to this VM")
	cmd.Flags().StringVar(&removeHook, "remove-hook", "", "Remove a hook from this VM by hook name")
	cmd.Flags().BoolVar(&clearHooks, "clear-hooks", false, "Remove all hooks from this VM")

	// Additional VM flags
	cmd.Flags().BoolVar(&deleteVmOnFailMigration, "delete-vm-on-fail-migration", false, "Delete target VM when migration fails (overrides plan-level setting)")

	// Add completion for hook flags
	if err := cmd.RegisterFlagCompletionFunc("add-pre-hook", completion.HookResourceNameCompletion(kubeConfigFlags)); err != nil {
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc("add-post-hook", completion.HookResourceNameCompletion(kubeConfigFlags)); err != nil {
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc("remove-hook", completion.HookResourceNameCompletion(kubeConfigFlags)); err != nil {
		panic(err)
	}

	// Add completion for target power state flag
	if err := cmd.RegisterFlagCompletionFunc("target-power-state", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"on", "off", "auto"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
