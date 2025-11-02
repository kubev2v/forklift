package create

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/vddk"
)

// NewVddkCmd creates the VDDK image creation command
func NewVddkCmd(globalConfig GlobalConfigGetter) *cobra.Command {
	var vddkTarGz, vddkTag, vddkBuildDir, vddkRuntime, vddkPlatform, vddkDockerfile string
	var vddkPush bool

	cmd := &cobra.Command{
		Use:   "vddk-image",
		Short: "Create a VDDK image for MTV",
		RunE: func(cmd *cobra.Command, args []string) error {
			verbosity := 0
			if globalConfig != nil {
				verbosity = globalConfig.GetVerbosity()
			}
			err := vddk.BuildImage(vddkTarGz, vddkTag, vddkBuildDir, vddkRuntime, vddkPlatform, vddkDockerfile, verbosity, vddkPush)
			if err != nil {
				fmt.Printf("Error building VDDK image: %v\n", err)
				fmt.Printf("You can use the '--help' flag for more information on usage.\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&vddkTarGz, "tar", "", "Path to VMware VDDK tar.gz file (required), e.g. VMware-vix-disklib.tar.gz")
	cmd.Flags().StringVar(&vddkTag, "tag", "", "Container image tag (required), e.g. quay.io/example/vddk:8.0.1")
	cmd.Flags().StringVar(&vddkBuildDir, "build-dir", "", "Build directory (optional, uses tmp dir if not set)")
	cmd.Flags().StringVar(&vddkRuntime, "runtime", "auto", "Container runtime to use: auto, podman, or docker (default: auto)")
	cmd.Flags().StringVar(&vddkPlatform, "platform", "amd64", "Target platform for the image: amd64 or arm64. (default: amd64)")
	cmd.Flags().StringVar(&vddkDockerfile, "dockerfile", "", "Path to custom Dockerfile (optional, uses default if not set)")
	cmd.Flags().BoolVar(&vddkPush, "push", false, "Push image after build (optional)")

	// Add autocomplete for runtime flag
	if err := cmd.RegisterFlagCompletionFunc("runtime", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"auto", "podman", "docker"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	// Add autocomplete for platform flag
	if err := cmd.RegisterFlagCompletionFunc("platform", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"amd64", "arm64"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	if err := cmd.MarkFlagRequired("tar"); err != nil {
		panic(err)
	}
	if err := cmd.MarkFlagRequired("tag"); err != nil {
		panic(err)
	}

	return cmd
}
