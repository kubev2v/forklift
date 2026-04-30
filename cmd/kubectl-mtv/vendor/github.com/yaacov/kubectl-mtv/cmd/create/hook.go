package create

import (
	"fmt"
	"os"
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/hook"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// NewHookCmd creates the hook creation command
func NewHookCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var name, image string
	var serviceAccount string
	var playbook string
	var deadline int64
	var dryRun bool
	var outputFormat string
	var aapJobTemplateID int

	// HookSpec fields
	var hookSpec forkliftv1beta1.HookSpec

	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Create a migration hook",
		Long: `Create a migration hook resource that can be used to run custom automation during migrations.

Hooks can be either local (image-based) or AAP (Ansible Automation Platform) hooks:

  Local hooks run a container image with an optional Ansible playbook.
  AAP hooks trigger a job template on the configured AAP server.

The two modes are mutually exclusive: specify --aap-job-template-id for AAP hooks,
or --image/--playbook for local hooks.

The playbook parameter supports the @ convention to read Ansible playbook content from a file.

Examples:
  # Create a local hook with default image and inline playbook content
  kubectl-mtv create hook --name my-hook --playbook "$(cat playbook.yaml)"

  # Create a local hook with custom image reading playbook from file
  kubectl-mtv create hook --name my-hook --image my-registry/hook-image:latest --playbook @playbook.yaml

  # Create an AAP hook that triggers job template 42
  kubectl-mtv create hook --name my-aap-hook --aap-job-template-id 42

  # Create a local hook with service account and deadline (uses default image)
  kubectl-mtv create hook --name my-hook --service-account my-sa --deadline 300`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if deadline < 0 {
				return fmt.Errorf("deadline must be a positive number")
			}

			if cmd.Flag("aap-job-template-id").Changed && aapJobTemplateID <= 0 {
				return fmt.Errorf("--aap-job-template-id must be a positive integer")
			}

			isAAP := aapJobTemplateID > 0
			imageChanged := cmd.Flag("image").Changed
			playbookChanged := cmd.Flag("playbook").Changed

			if isAAP && (imageChanged || playbookChanged) {
				return fmt.Errorf("--aap-job-template-id is mutually exclusive with --image and --playbook")
			}

			namespace := client.ResolveNamespace(kubeConfigFlags)

			if strings.HasPrefix(playbook, "@") {
				filePath := playbook[1:]
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read playbook file %s: %v", filePath, err)
				}
				playbook = string(fileContent)
			}

			if !isAAP {
				if !imageChanged {
					image = "quay.io/kubev2v/hook-runner"
				}
				hookSpec.Image = image
			}
			if serviceAccount != "" {
				hookSpec.ServiceAccount = serviceAccount
			}
			if playbook != "" {
				hookSpec.Playbook = playbook
			}
			if deadline > 0 {
				hookSpec.Deadline = deadline
			}

			if !dryRun && outputFormat != "" {
				return fmt.Errorf("--output flag can only be used with --dry-run")
			}
			if dryRun && outputFormat != "" && outputFormat != "json" && outputFormat != "yaml" {
				return fmt.Errorf("invalid output format for dry-run: %s. Valid formats are: json, yaml", outputFormat)
			}
			resolvedFormat := outputFormat
			if dryRun && resolvedFormat == "" {
				resolvedFormat = "yaml"
			}

			opts := hook.CreateHookOptions{
				Name:             name,
				Namespace:        namespace,
				ConfigFlags:      kubeConfigFlags,
				HookSpec:         hookSpec,
				DryRun:           dryRun,
				OutputFormat:     resolvedFormat,
				AAPJobTemplateID: aapJobTemplateID,
			}

			return hook.Create(opts)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Hook name")
	cmd.Flags().StringVar(&image, "image", "", "Container image URL to run (default: quay.io/kubev2v/hook-runner for local hooks)")
	cmd.Flags().StringVar(&serviceAccount, "service-account", "", "Service account to use for the hook (optional)")
	cmd.Flags().StringVar(&playbook, "playbook", "", "Ansible playbook content, or use @filename to read from file (optional)")
	cmd.Flags().Int64Var(&deadline, "deadline", 0, "Hook deadline in seconds (optional)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Output Hook CR to stdout instead of creating it")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format for dry-run (json, yaml). Defaults to yaml when --dry-run is used")
	cmd.Flags().IntVar(&aapJobTemplateID, "aap-job-template-id", 0, "AAP job template ID (mutually exclusive with --image and --playbook)")

	if err := cmd.MarkFlagRequired("name"); err != nil {
		panic(err)
	}

	return cmd
}
