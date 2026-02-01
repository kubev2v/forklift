package create

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewProviderCmd creates the provider creation command
func NewProviderCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var secret string
	providerType := flags.NewProviderTypeFlag()

	// Add Provider credential flags
	var url, username, password, cacert, token string
	var insecureSkipTLS bool
	var vddkInitImage string
	sdkEndpointType := flags.NewSdkEndpointTypeFlag()

	// VSphere VDDK specific flags
	var useVddkAioOptimization bool
	var vddkBufSizeIn64K, vddkBufCount int

	// OpenStack specific flags
	var domainName, projectName, regionName string

	// EC2 specific flags
	var ec2Region, ec2TargetRegion, ec2TargetAZ string
	var ec2TargetAccessKeyID, ec2TargetSecretKey string
	var autoTargetCredentials bool

	// Check if MTV_VDDK_INIT_IMAGE environment variable is set
	if envVddkInitImage := os.Getenv("MTV_VDDK_INIT_IMAGE"); envVddkInitImage != "" {
		vddkInitImage = envVddkInitImage
	}

	cmd := &cobra.Command{
		Use:   "provider NAME",
		Short: "Create a new provider",
		Long: `Create a new MTV provider to connect to a virtualization platform.

Providers represent source or target environments for VM migrations. Supported types:
  - vsphere: VMware vSphere/vCenter (requires VDDK init image for migration)
  - ovirt: Red Hat Virtualization (oVirt/RHV)
  - openstack: OpenStack cloud platform
  - ova: OVA files from NFS share
  - openshift: Target OpenShift cluster (usually named 'host')
  - ec2: Amazon EC2 instances

Credentials can be provided directly via flags or through an existing Kubernetes secret.`,
		Example: `  # Create a vSphere provider
  kubectl-mtv create provider vsphere-prod \
    --type vsphere \
    --url https://vcenter.example.com/sdk \
    --username admin@vsphere.local \
    --password 'secret' \
    --vddk-init-image quay.io/kubev2v/vddk:latest

  # Create an oVirt provider
  kubectl-mtv create provider ovirt-prod \
    --type ovirt \
    --url https://rhv-manager.example.com/ovirt-engine/api \
    --username admin@internal \
    --password 'secret'

  # Create an OpenShift target provider
  kubectl-mtv create provider host \
    --type openshift \
    --url https://api.cluster.example.com:6443 \
    --token 'eyJhbGciOiJSUzI1NiIsInR5...'

  # Create an OpenStack provider
  kubectl-mtv create provider openstack-prod \
    --type openstack \
    --url https://keystone.example.com:5000/v3 \
    --username admin \
    --password 'secret' \
    --provider-domain-name Default \
    --provider-project-name admin`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Fetch dynamic provider types from the cluster
			dynamicTypes, err := client.GetDynamicProviderTypes(kubeConfigFlags)
			if err != nil {
				// Log the error but don't fail - we can still work with static types
				// This allows the command to work even if there are cluster connectivity issues
				// as long as the user is using a static provider type
				cmd.PrintErrf("Warning: failed to fetch dynamic provider types: %v\n", err)
			} else {
				// Set the dynamic types in the flag
				providerType.SetDynamicTypes(dynamicTypes)
			}
			return nil
		},
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
					return err
				}
				cacert = string(fileContent)
			}

			return provider.Create(kubeConfigFlags, providerType.GetValue(), name, namespace, secret,
				url, username, password, cacert, insecureSkipTLS, vddkInitImage, sdkEndpointType.GetValue(), token,
				domainName, projectName, regionName, useVddkAioOptimization, vddkBufSizeIn64K, vddkBufCount,
				ec2Region, ec2TargetRegion, ec2TargetAZ, ec2TargetAccessKeyID, ec2TargetSecretKey, autoTargetCredentials)
		},
	}

	cmd.Flags().VarP(providerType, "type", "t", "Provider type (openshift, vsphere, ovirt, openstack, ova, ec2)")
	cmd.Flags().StringVar(&secret, "secret", "", "Secret containing provider credentials")

	// Provider credential flags
	cmd.Flags().StringVarP(&url, "url", "U", "", "Provider URL")
	cmd.Flags().StringVarP(&username, "username", "u", "", "Provider credentials username")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Provider credentials password")
	cmd.Flags().StringVar(&cacert, "cacert", "", "Provider CA certificate (use @filename to load from file)")
	cmd.Flags().BoolVar(&insecureSkipTLS, "provider-insecure-skip-tls", false, "Skip TLS verification when connecting to the provider")

	// OpenShift specific flags
	cmd.Flags().StringVarP(&token, "token", "T", "", "Provider authentication token "+flags.ProvidersOpenShift)

	// vSphere specific flags
	cmd.Flags().StringVar(&vddkInitImage, "vddk-init-image", vddkInitImage, "Virtual Disk Development Kit (VDDK) container init image path "+flags.ProvidersVSphere)
	cmd.Flags().Var(sdkEndpointType, "sdk-endpoint", "SDK endpoint type (vcenter or esxi) "+flags.ProvidersVSphere)
	cmd.Flags().BoolVar(&useVddkAioOptimization, "use-vddk-aio-optimization", false, "Enable VDDK AIO optimization for improved disk transfer performance "+flags.ProvidersVSphere)
	cmd.Flags().IntVar(&vddkBufSizeIn64K, "vddk-buf-size-in-64k", 0, "VDDK buffer size in 64K units (VixDiskLib.nfcAio.Session.BufSizeIn64K) "+flags.ProvidersVSphere)
	cmd.Flags().IntVar(&vddkBufCount, "vddk-buf-count", 0, "VDDK buffer count (VixDiskLib.nfcAio.Session.BufCount) "+flags.ProvidersVSphere)

	// OpenStack specific flags
	cmd.Flags().StringVar(&domainName, "provider-domain-name", "", "OpenStack domain name "+flags.ProvidersOpenStack)
	cmd.Flags().StringVar(&projectName, "provider-project-name", "", "OpenStack project name "+flags.ProvidersOpenStack)
	cmd.Flags().StringVar(&regionName, "provider-region-name", "", "OpenStack region name "+flags.ProvidersOpenStack)
	cmd.Flags().StringVar(&regionName, "region", "", "Region name (alias for --provider-region-name) "+flags.ProviderHint("openstack", "ec2"))

	// EC2 specific flags
	cmd.Flags().StringVar(&ec2Region, "ec2-region", "", "AWS region where source EC2 instances are located "+flags.ProvidersEC2)
	cmd.Flags().StringVar(&ec2TargetRegion, "target-region", "", "Target region for migrations (defaults to provider region) "+flags.ProvidersEC2)
	cmd.Flags().StringVar(&ec2TargetAZ, "target-az", "", "Target availability zone for migrations (required - EBS volumes are AZ-specific) "+flags.ProvidersEC2)
	cmd.Flags().StringVar(&ec2TargetAccessKeyID, "target-access-key-id", "", "Target AWS account access key ID (for cross-account migrations) "+flags.ProvidersEC2)
	cmd.Flags().StringVar(&ec2TargetSecretKey, "target-secret-access-key", "", "Target AWS account secret access key (for cross-account migrations) "+flags.ProvidersEC2)
	cmd.Flags().BoolVar(&autoTargetCredentials, "auto-target-credentials", false, "Automatically fetch target AWS credentials from cluster and target-az from worker nodes "+flags.ProvidersEC2)

	// Add completion for provider type flag
	if err := cmd.RegisterFlagCompletionFunc("type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return providerType.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	// Add completion for sdk-endpoint flag
	if err := cmd.RegisterFlagCompletionFunc("sdk-endpoint", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return sdkEndpointType.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	if err := cmd.MarkFlagRequired("type"); err != nil {
		panic(err)
	}

	return cmd
}
