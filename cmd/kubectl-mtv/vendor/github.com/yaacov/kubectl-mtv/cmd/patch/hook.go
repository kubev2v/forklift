package patch

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/patch/hook"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewHookCmd creates the patch hook command
func NewHookCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	opts := hook.PatchHookOptions{
		ConfigFlags: kubeConfigFlags,
	}

	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Patch an existing migration hook",
		Long: `Patch an existing migration hook by updating its configuration.

Hooks can be either local (image-based) or AAP (Ansible Automation Platform) hooks.
Use --aap-job-template-id to set/change AAP configuration, or --clear-aap to remove it.

Examples:
  # Update the image of a local hook
  kubectl-mtv patch hook --name my-hook --image my-registry/hook-image:v2

  # Switch a local hook to an AAP hook
  kubectl-mtv patch hook --name my-hook --aap-job-template-id 42 --image ""

  # Switch an AAP hook back to a local hook
  kubectl-mtv patch hook --name my-hook --clear-aap --image quay.io/kubev2v/hook-runner

  # Update the deadline
  kubectl-mtv patch hook --name my-hook --deadline 600`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.ResolveNameArg(&opts.Name, args); err != nil {
				return err
			}
			if opts.Name == "" {
				return fmt.Errorf("--name is required")
			}

			opts.Namespace = client.ResolveNamespace(kubeConfigFlags)

			opts.ImageChanged = cmd.Flag("image").Changed
			opts.SAChanged = cmd.Flag("service-account").Changed
			opts.PlaybookChanged = cmd.Flag("playbook").Changed
			opts.DeadlineChanged = cmd.Flag("deadline").Changed
			opts.AAPJobTemplateIDChanged = cmd.Flag("aap-job-template-id").Changed

			if opts.PlaybookChanged && strings.HasPrefix(opts.Playbook, "@") {
				filePath := opts.Playbook[1:]
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read playbook file %s: %v", filePath, err)
				}
				opts.Playbook = string(fileContent)
			}

			return hook.PatchHook(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Name, "name", "M", "", "Hook name")
	flags.MarkRequiredForMCP(cmd, "name")

	cmd.Flags().StringVar(&opts.Image, "image", "", "Container image URL")
	cmd.Flags().StringVar(&opts.ServiceAccount, "service-account", "", "Service account")
	cmd.Flags().StringVar(&opts.Playbook, "playbook", "", "Ansible playbook content, or use @filename to read from file")
	cmd.Flags().Int64Var(&opts.Deadline, "deadline", 0, "Hook deadline in seconds")
	cmd.Flags().IntVar(&opts.AAPJobTemplateID, "aap-job-template-id", 0, "AAP job template ID")
	cmd.Flags().BoolVar(&opts.ClearAAP, "clear-aap", false, "Remove AAP configuration from the hook")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.HookResourceNameCompletion(kubeConfigFlags))

	return cmd
}
