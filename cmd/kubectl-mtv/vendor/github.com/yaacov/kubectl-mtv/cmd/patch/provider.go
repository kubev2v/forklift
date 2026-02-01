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
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewProviderCmd creates the patch provider command
func NewProviderCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	opts := provider.PatchProviderOptions{
		ConfigFlags: kubeConfigFlags,
	}

	// Check if MTV_VDDK_INIT_IMAGE environment variable is set
	if envVddkInitImage := os.Getenv("MTV_VDDK_INIT_IMAGE"); envVddkInitImage != "" {
		opts.VddkInitImage = envVddkInitImage
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
			opts.Name = args[0]

			// Resolve the appropriate namespace based on context and flags
			opts.Namespace = client.ResolveNamespace(kubeConfigFlags)

			// Check if cacert starts with @ and load from file if so
			if strings.HasPrefix(opts.CACert, "@") {
				filePath := opts.CACert[1:]
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read CA certificate file '%s': %v", filePath, err)
				}
				opts.CACert = string(fileContent)
			}

			// Set flag change tracking
			opts.InsecureSkipTLSChanged = cmd.Flag("provider-insecure-skip-tls").Changed
			opts.UseVddkAioOptimizationChanged = cmd.Flag("use-vddk-aio-optimization").Changed

			return provider.PatchProvider(opts)
		},
	}

	// Editable provider flags
	cmd.Flags().StringVarP(&opts.URL, "url", "U", "", "Provider URL")
	cmd.Flags().StringVarP(&opts.Username, "username", "u", "", "Provider credentials username")
	cmd.Flags().StringVarP(&opts.Password, "password", "p", "", "Provider credentials password")
	cmd.Flags().StringVar(&opts.CACert, "cacert", "", "Provider CA certificate (use @filename to load from file)")
	cmd.Flags().BoolVar(&opts.InsecureSkipTLS, "provider-insecure-skip-tls", false, "Skip TLS verification when connecting to the provider")

	// OpenShift specific flags
	cmd.Flags().StringVarP(&opts.Token, "token", "T", "", "Provider authentication token "+flags.ProvidersOpenShift)

	// vSphere specific flags (editable VDDK settings)
	cmd.Flags().StringVar(&opts.VddkInitImage, "vddk-init-image", "", "Virtual Disk Development Kit (VDDK) container init image path "+flags.ProvidersVSphere)
	cmd.Flags().BoolVar(&opts.UseVddkAioOptimization, "use-vddk-aio-optimization", false, "Enable VDDK AIO optimization for improved disk transfer performance "+flags.ProvidersVSphere)
	cmd.Flags().IntVar(&opts.VddkBufSizeIn64K, "vddk-buf-size-in-64k", 0, "VDDK buffer size in 64K units (VixDiskLib.nfcAio.Session.BufSizeIn64K) "+flags.ProvidersVSphere)
	cmd.Flags().IntVar(&opts.VddkBufCount, "vddk-buf-count", 0, "VDDK buffer count (VixDiskLib.nfcAio.Session.BufCount) "+flags.ProvidersVSphere)

	// OpenStack specific flags
	cmd.Flags().StringVar(&opts.DomainName, "provider-domain-name", "", "OpenStack domain name "+flags.ProvidersOpenStack)
	cmd.Flags().StringVar(&opts.ProjectName, "provider-project-name", "", "OpenStack project name "+flags.ProvidersOpenStack)
	cmd.Flags().StringVar(&opts.RegionName, "provider-region-name", "", "OpenStack region name "+flags.ProvidersOpenStack)
	cmd.Flags().StringVar(&opts.RegionName, "region", "", "Region name (alias for --provider-region-name) "+flags.ProviderHint("openstack", "ec2"))

	// EC2 specific flags
	cmd.Flags().StringVar(&opts.EC2Region, "ec2-region", "", "AWS region where source EC2 instances are located "+flags.ProvidersEC2)
	cmd.Flags().StringVar(&opts.EC2TargetRegion, "target-region", "", "Target region for migrations (defaults to provider region) "+flags.ProvidersEC2)
	cmd.Flags().StringVar(&opts.EC2TargetAZ, "target-az", "", "Target availability zone for migrations (required - EBS volumes are AZ-specific) "+flags.ProvidersEC2)
	cmd.Flags().StringVar(&opts.EC2TargetAccessKeyID, "target-access-key-id", "", "Target AWS account access key ID (for cross-account migrations) "+flags.ProvidersEC2)
	cmd.Flags().StringVar(&opts.EC2TargetSecretKey, "target-secret-access-key", "", "Target AWS account secret access key (for cross-account migrations) "+flags.ProvidersEC2)
	cmd.Flags().BoolVar(&opts.AutoTargetCredentials, "auto-target-credentials", false, "Automatically fetch target AWS credentials from cluster and target-az from worker nodes "+flags.ProvidersEC2)

	return cmd
}
