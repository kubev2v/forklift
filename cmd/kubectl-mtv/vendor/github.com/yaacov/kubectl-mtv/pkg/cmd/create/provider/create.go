package provider

import (
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/azure"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/ec2"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/generic"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/hyperv"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/openshift"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/openstack"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/ova"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/providerutil"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/vsphere"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// Create creates a new provider
func Create(configFlags *genericclioptions.ConfigFlags, providerType string, options providerutil.ProviderOptions) error {
	// For EC2 provider, use regionName (from --provider-region-name) if ec2Region is empty
	// This allows using --provider-region-name for EC2 regions as shown in documentation
	if providerType == "ec2" && options.EC2Region == "" && options.RegionName != "" {
		options.EC2Region = options.RegionName
	}

	var providerResource *forkliftv1beta1.Provider
	var secretResource *corev1.Secret
	var err error

	// Create the provider and secret based on the specified type
	switch providerType {
	case "vsphere":
		providerResource, secretResource, err = vsphere.CreateProvider(configFlags, options)
	case "ova":
		providerResource, secretResource, err = ova.CreateProvider(configFlags, options)
	case "hyperv":
		providerResource, secretResource, err = hyperv.CreateProvider(configFlags, options)
	case "openshift":
		providerResource, secretResource, err = openshift.CreateProvider(configFlags, options)
	case "ovirt":
		providerResource, secretResource, err = generic.CreateProvider(configFlags, options, "ovirt")
	case "openstack":
		providerResource, secretResource, err = openstack.CreateProvider(configFlags, options)
	case "ec2":
		providerResource, secretResource, err = ec2.CreateProvider(configFlags, options)
	case string(flags.AzureProviderType):
		providerResource, secretResource, err = azure.CreateProvider(configFlags, options)
	case string(forkliftv1beta1.Nutanix):
		if err := validateNutanixOptions(options); err != nil {
			return err
		}
		options.Settings = nutanixSettings(options)
		providerResource, secretResource, err = generic.CreateProvider(configFlags, options, string(forkliftv1beta1.Nutanix))
	default:
		return fmt.Errorf("unsupported provider type: %s", providerType)
	}

	// Handle any errors that occurred during provider creation
	if err != nil {
		return fmt.Errorf("failed to prepare provider: %v", err)
	}

	if options.DryRun {
		if secretResource != nil {
			if err := output.OutputResource(secretResource, options.OutputFormat); err != nil {
				return err
			}
		}
		return output.OutputResource(providerResource, options.OutputFormat)
	}

	// Display the creation results to the user
	fmt.Printf("provider/%s created\n", providerResource.Name)

	if secretResource != nil {
		fmt.Printf("Created secret '%s' for provider authentication\n", secretResource.Name)
	} else if options.Secret != "" {
		fmt.Printf("Using existing secret '%s' for provider authentication\n", options.Secret)
	}

	return nil
}

func validateNutanixOptions(options providerutil.ProviderOptions) error {
	if options.NutanixPrismType == "" {
		return nil
	}
	switch options.NutanixPrismType {
	case string(forkliftv1beta1.NutanixPrismCentral), string(forkliftv1beta1.NutanixPrismElement):
		return nil
	default:
		return fmt.Errorf("invalid --nutanix-prism-type %q: must be %s or %s",
			options.NutanixPrismType, forkliftv1beta1.NutanixPrismCentral, forkliftv1beta1.NutanixPrismElement)
	}
}

func nutanixSettings(options providerutil.ProviderOptions) map[string]string {
	settings := map[string]string{}
	if options.NutanixPrismType != "" {
		settings[forkliftv1beta1.NutanixPrismType] = options.NutanixPrismType
	}
	if options.NutanixClusterUUID != "" {
		settings[forkliftv1beta1.NutanixClusterUUID] = options.NutanixClusterUUID
	}
	if len(settings) == 0 {
		return nil
	}
	return settings
}
