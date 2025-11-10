package patch

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/patch/provider"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewProviderCmd creates the patch provider command
func NewProviderCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	// Provider credential flags (editable)
	var url, username, password, cacert, token string
	var insecureSkipTLS bool
	var vddkInitImage string

	// VSphere VDDK specific flags (editable)
	var useVddkAioOptimization bool
	var vddkBufSizeIn64K, vddkBufCount int

	// OpenStack specific flags (editable)
	var domainName, projectName, regionName string

	// Check if MTV_VDDK_INIT_IMAGE environment variable is set
	if envVddkInitImage := os.Getenv("MTV_VDDK_INIT_IMAGE"); envVddkInitImage != "" {
		vddkInitImage = envVddkInitImage
	}

	cmd := &cobra.Command{
		Use:               "provider NAME",
		Short:             "Patch an existing provider",
		Long:              `Patch an existing provider by updating URL, credentials, or VDDK settings. Type and SDK endpoint cannot be changed.`,
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.ProviderNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get name from positional argument
			name := args[0]

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			// Check if cacert starts with @ and load from file if so
			if strings.HasPrefix(cacert, "@") {
				filePath := cacert[1:]
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read CA certificate file '%s': %v", filePath, err)
				}
				cacert = string(fileContent)
			}

			return provider.PatchProvider(kubeConfigFlags, name, namespace,
				url, username, password, cacert, insecureSkipTLS, vddkInitImage, token,
				domainName, projectName, regionName, useVddkAioOptimization, vddkBufSizeIn64K, vddkBufCount,
				cmd.Flag("provider-insecure-skip-tls").Changed, cmd.Flag("use-vddk-aio-optimization").Changed)
		},
	}

	// Editable provider flags
	cmd.Flags().StringVarP(&url, "url", "U", "", "Provider URL")
	cmd.Flags().StringVarP(&username, "username", "u", "", "Provider credentials username")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Provider credentials password")
	cmd.Flags().StringVar(&cacert, "cacert", "", "Provider CA certificate (use @filename to load from file)")
	cmd.Flags().BoolVar(&insecureSkipTLS, "provider-insecure-skip-tls", false, "Skip TLS verification when connecting to the provider")

	// OpenShift specific flags
	cmd.Flags().StringVarP(&token, "token", "T", "", "Provider authentication token (used for openshift provider)")

	// VSphere specific flags (editable VDDK settings)
	cmd.Flags().StringVar(&vddkInitImage, "vddk-init-image", "", "Virtual Disk Development Kit (VDDK) container init image path")
	cmd.Flags().BoolVar(&useVddkAioOptimization, "use-vddk-aio-optimization", false, "Enable VDDK AIO optimization for vSphere provider")
	cmd.Flags().IntVar(&vddkBufSizeIn64K, "vddk-buf-size-in-64k", 0, "VDDK buffer size in 64K units (VixDiskLib.nfcAio.Session.BufSizeIn64K)")
	cmd.Flags().IntVar(&vddkBufCount, "vddk-buf-count", 0, "VDDK buffer count (VixDiskLib.nfcAio.Session.BufCount)")

	// OpenStack specific flags
	cmd.Flags().StringVar(&domainName, "provider-domain-name", "", "OpenStack domain name")
	cmd.Flags().StringVar(&projectName, "provider-project-name", "", "OpenStack project name")
	cmd.Flags().StringVar(&regionName, "provider-region-name", "", "OpenStack region name")

	return cmd
}
