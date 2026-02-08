package create

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/vddk"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewVddkCmd creates the VDDK image creation command
func NewVddkCmd(globalConfig GlobalConfigGetter, kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var vddkTarGz, vddkTag, vddkBuildDir, vddkRuntime, vddkPlatform, vddkDockerfile string
	var vddkPush, setControllerImage, vddkPushInsecureSkipTLS bool

	cmd := &cobra.Command{
		Use:   "vddk-image",
		Short: "Create a VDDK image for MTV " + flags.ProvidersVSphere,
		Long: `Build a VDDK (Virtual Disk Development Kit) container image for vSphere migrations.

VDDK is required for migrating VMs from vSphere. This command builds a container
image from the VMware VDDK SDK and pushes it to your container registry.

You must download the VDDK SDK from VMware (requires VMware account):
https://developer.vmware.com/web/sdk/8.0/vddk`,
		Example: `  # Build VDDK image using podman
  kubectl-mtv create vddk-image \
    --tar VMware-vix-disklib-8.0.1-21562716.x86_64.tar.gz \
    --tag quay.io/myorg/vddk:8.0.1

  # Build and push to registry
  kubectl-mtv create vddk-image \
    --tar VMware-vix-disklib-8.0.1-21562716.x86_64.tar.gz \
    --tag quay.io/myorg/vddk:8.0.1 \
    --push

  # Build, push, and configure as global VDDK image in ForkliftController
  kubectl-mtv create vddk-image \
    --tar VMware-vix-disklib-8.0.1-21562716.x86_64.tar.gz \
    --tag quay.io/myorg/vddk:8.0.1 \
    --push \
    --set-controller-image

  # Use specific container runtime
  kubectl-mtv create vddk-image \
    --tar VMware-vix-disklib-8.0.1-21562716.x86_64.tar.gz \
    --tag quay.io/myorg/vddk:8.0.1 \
    --runtime docker

  # Push to insecure registry (self-signed certificate)
  kubectl-mtv create vddk-image \
    --tar VMware-vix-disklib-8.0.1-21562716.x86_64.tar.gz \
    --tag internal-registry.local:5000/vddk:8.0.1 \
    --push \
    --push-insecure-skip-tls`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate that --set-controller-image requires --push
			if setControllerImage && !vddkPush {
				return fmt.Errorf("--set-controller-image requires --push to be set")
			}

			verbosity := 0
			if globalConfig != nil {
				verbosity = globalConfig.GetVerbosity()
			}
			err := vddk.BuildImage(vddkTarGz, vddkTag, vddkBuildDir, vddkRuntime, vddkPlatform, vddkDockerfile, verbosity, vddkPush, vddkPushInsecureSkipTLS)
			if err != nil {
				fmt.Printf("Error building VDDK image: %v\n", err)
				fmt.Printf("You can use the '--help' flag for more information on usage.\n")
				return nil
			}

			// Configure ForkliftController if requested
			if setControllerImage {
				if err := vddk.SetControllerVddkImage(kubeConfigFlags, vddkTag, verbosity); err != nil {
					fmt.Printf("Error configuring ForkliftController: %v\n", err)
					return nil
				}
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
	cmd.Flags().BoolVar(&vddkPushInsecureSkipTLS, "push-insecure-skip-tls", false, "Skip TLS verification when pushing to the registry (podman only, docker requires daemon config)")
	cmd.Flags().BoolVar(&setControllerImage, "set-controller-image", false, "Configure the pushed image as global vddk_image in ForkliftController (requires --push)")

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
