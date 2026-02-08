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
	var image string
	var serviceAccount string
	var playbook string
	var deadline int64

	// HookSpec fields
	var hookSpec forkliftv1beta1.HookSpec

	cmd := &cobra.Command{
		Use:   "hook NAME",
		Short: "Create a migration hook",
		Long: `Create a migration hook resource that can be used to run custom automation during migrations.

Hooks allow you to execute custom logic at various points during the migration process by running 
container images with Ansible playbooks. Hooks can be used for pre-migration validation, 
post-migration cleanup, or any custom automation needs.

The playbook parameter supports the @ convention to read Ansible playbook content from a file.

Examples:
  # Create a hook with default image and inline playbook content
  kubectl-mtv create hook my-hook --playbook "$(cat playbook.yaml)"

  # Create a hook with custom image reading playbook from file
  kubectl-mtv create hook my-hook --image my-registry/hook-image:latest --playbook @playbook.yaml

  # Create a hook with service account and deadline (uses default image)
  kubectl-mtv create hook my-hook --service-account my-sa --deadline 300`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get name from positional argument
			name := args[0]

			// Validate deadline is positive
			if deadline < 0 {
				return fmt.Errorf("deadline must be a positive number")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			// Handle playbook file loading if @ convention is used
			if strings.HasPrefix(playbook, "@") {
				filePath := playbook[1:]
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read playbook file %s: %v", filePath, err)
				}
				playbook = string(fileContent)
			}

			// Set the HookSpec fields
			hookSpec.Image = image
			if serviceAccount != "" {
				hookSpec.ServiceAccount = serviceAccount
			}
			if playbook != "" {
				hookSpec.Playbook = playbook
			}
			if deadline > 0 {
				hookSpec.Deadline = deadline
			}

			opts := hook.CreateHookOptions{
				Name:        name,
				Namespace:   namespace,
				ConfigFlags: kubeConfigFlags,
				HookSpec:    hookSpec,
			}

			return hook.Create(opts)
		},
	}

	cmd.Flags().StringVar(&image, "image", "quay.io/kubev2v/hook-runner", "Container image URL to run (default: quay.io/kubev2v/hook-runner)")
	cmd.Flags().StringVar(&serviceAccount, "service-account", "", "Service account to use for the hook (optional)")
	cmd.Flags().StringVar(&playbook, "playbook", "", "Ansible playbook content, or use @filename to read from file (optional)")
	cmd.Flags().Int64Var(&deadline, "deadline", 0, "Hook deadline in seconds (optional)")

	return cmd
}
