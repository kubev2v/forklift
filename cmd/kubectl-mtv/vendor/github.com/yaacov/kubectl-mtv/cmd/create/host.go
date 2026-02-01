package create

import (
	"fmt"
	"os"
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/host"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewHostCmd creates the host creation command
func NewHostCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	var provider string
	var username, password string
	var existingSecret string
	var ipAddress string
	var networkAdapterName string
	var hostInsecureSkipTLS bool
	var cacert string

	// HostSpec fields
	var hostSpec forkliftv1beta1.HostSpec

	cmd := &cobra.Command{
		Use:   "host NAME [NAME...]",
		Short: "Create migration hosts " + flags.ProvidersVSphere,
		Long: `Create migration hosts for vSphere providers. Hosts enable direct data transfer from ESXi hosts, bypassing vCenter for improved performance.

By creating host resources, Forklift can utilize ESXi host interfaces directly for network transfer to OpenShift, provided the OpenShift worker nodes and ESXi host interfaces have network connectivity. This is particularly beneficial when users want to control which specific ESXi interface is used for migration, even without direct access to ESXi host credentials.

Only vSphere providers support host creation. Host names must match existing hosts in the provider's inventory.

Examples:
  # ESXi endpoint provider with direct IP (no credentials needed - uses provider secret automatically)
  kubectl-mtv create host my-host --provider my-esxi-provider --ip-address 192.168.1.10

  # ESXi endpoint provider with network adapter lookup
  kubectl-mtv create host my-host --provider my-esxi-provider --network-adapter "Management Network"

  # Create a host using existing secret and direct IP
  kubectl-mtv create host my-host --provider my-vsphere-provider --existing-secret my-secret --ip-address 192.168.1.10

  # Create a host with new credentials and direct IP
  kubectl-mtv create host my-host --provider my-vsphere-provider --username user --password pass --ip-address 192.168.1.10

  # Create a host using IP from inventory network adapter
  kubectl-mtv create host my-host --provider my-vsphere-provider --username user --password pass --network-adapter "Management Network"

  # Create multiple hosts (all use same IP resolution method)
  kubectl-mtv create host host1 host2 host3 --provider my-vsphere-provider --existing-secret my-secret --network-adapter "Management Network"`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate input parameters
			if provider == "" {
				return fmt.Errorf("provider is required")
			}

			namespace := client.ResolveNamespace(kubeConfigFlags)

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			providerHasESXIEndpoint, _, err := host.CheckProviderESXIEndpoint(cmd.Context(), kubeConfigFlags, provider, namespace)
			if err != nil {
				return fmt.Errorf("failed to check provider endpoint type: %v", err)
			}

			if !providerHasESXIEndpoint {
				if existingSecret == "" && (username == "" || password == "") {
					return fmt.Errorf("either --existing-secret OR both --username and --password must be provided")
				}
			}

			if existingSecret != "" && (username != "" || password != "") {
				return fmt.Errorf("cannot use both --existing-secret and --username/--password")
			}

			if ipAddress == "" && networkAdapterName == "" {
				return fmt.Errorf("either --ip-address OR --network-adapter must be provided")
			}
			if ipAddress != "" && networkAdapterName != "" {
				return fmt.Errorf("cannot use both --ip-address and --network-adapter")
			}

			if strings.HasPrefix(cacert, "@") {
				filePath := cacert[1:]
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read CA certificate file %s: %v", filePath, err)
				}
				cacert = string(fileContent)
			}

			hostIDs := args

			opts := host.CreateHostOptions{
				HostIDs:                  hostIDs,
				Namespace:                namespace,
				Provider:                 provider,
				ConfigFlags:              kubeConfigFlags,
				InventoryURL:             inventoryURL,
				InventoryInsecureSkipTLS: inventoryInsecureSkipTLS,
				Username:                 username,
				Password:                 password,
				ExistingSecret:           existingSecret,
				IPAddress:                ipAddress,
				NetworkAdapterName:       networkAdapterName,
				HostInsecureSkipTLS:      hostInsecureSkipTLS,
				CACert:                   cacert,
				HostSpec:                 hostSpec,
			}

			return host.Create(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Provider name (must be a vSphere provider)")
	cmd.Flags().StringVarP(&username, "username", "u", "", "Username for host authentication (required if --existing-secret not provided)")
	cmd.Flags().StringVar(&password, "password", "", "Password for host authentication (required if --existing-secret not provided)")
	cmd.Flags().StringVar(&existingSecret, "existing-secret", "", "Name of existing secret to use for host authentication")
	cmd.Flags().StringVar(&ipAddress, "ip-address", "", "IP address for disk transfer (required - mutually exclusive with --network-adapter)")
	cmd.Flags().StringVar(&networkAdapterName, "network-adapter", "", "Network adapter name to get IP address from inventory (required - mutually exclusive with --ip-address)")
	cmd.Flags().BoolVar(&hostInsecureSkipTLS, "host-insecure-skip-tls", false, "Skip TLS verification when connecting to the host (only used when creating new secret)")
	cmd.Flags().StringVar(&cacert, "cacert", "", "CA certificate for host authentication - provide certificate content directly or use @filename to load from file (only used when creating new secret)")

	if err := cmd.MarkFlagRequired("provider"); err != nil {
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc("provider", completion.ProviderNameCompletionByType(kubeConfigFlags, "vsphere")); err != nil {
		panic(err)
	}

	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completion.HostNameCompletion(kubeConfigFlags, provider, toComplete)
	}

	if err := cmd.RegisterFlagCompletionFunc("ip-address", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completion.HostIPAddressCompletion(kubeConfigFlags, provider, args, toComplete)
	}); err != nil {
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc("network-adapter", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completion.HostNetworkAdapterCompletion(kubeConfigFlags, provider, args, toComplete)
	}); err != nil {
		panic(err)
	}

	return cmd
}
