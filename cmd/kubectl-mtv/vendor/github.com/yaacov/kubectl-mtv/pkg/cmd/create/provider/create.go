package provider

import (
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/ec2"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/generic"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/hyperv"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/openshift"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/openstack"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/ova"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/providerutil"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/vsphere"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// Create creates a new provider
func Create(configFlags *genericclioptions.ConfigFlags, providerType, name, namespace, secret string,
	url, username, password, cacert string, insecureSkipTLS bool, vddkInitImage, sdkEndpoint string, token string,
	domainName, projectName, regionName string, useVddkAioOptimization bool, vddkBufSizeIn64K, vddkBufCount int,
	ec2Region, ec2TargetRegion, ec2TargetAZ, ec2TargetAccessKeyID, ec2TargetSecretKey string, autoTargetCredentials bool) error {
	// For EC2 provider, use regionName (from --provider-region-name) if ec2Region is empty
	// This allows using --provider-region-name for EC2 regions as shown in documentation
	if providerType == "ec2" && ec2Region == "" && regionName != "" {
		ec2Region = regionName
	}

	// Create provider options
	options := providerutil.ProviderOptions{
		Name:                   name,
		Namespace:              namespace,
		Secret:                 secret,
		URL:                    url,
		Username:               username,
		Password:               password,
		CACert:                 cacert,
		InsecureSkipTLS:        insecureSkipTLS,
		VddkInitImage:          vddkInitImage,
		SdkEndpoint:            sdkEndpoint,
		Token:                  token,
		DomainName:             domainName,
		ProjectName:            projectName,
		RegionName:             regionName,
		UseVddkAioOptimization: useVddkAioOptimization,
		VddkBufSizeIn64K:       vddkBufSizeIn64K,
		VddkBufCount:           vddkBufCount,
		EC2Region:              ec2Region,
		EC2TargetRegion:        ec2TargetRegion,
		EC2TargetAZ:            ec2TargetAZ,
		EC2TargetAccessKeyID:   ec2TargetAccessKeyID,
		EC2TargetSecretKey:     ec2TargetSecretKey,
		AutoTargetCredentials:  autoTargetCredentials,
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
	default:
		// For dynamic provider types, use generic provider creation
		// This allows support for DynamicProvider CRs defined in the cluster
		providerResource, secretResource, err = generic.CreateProvider(configFlags, options, providerType)
	}

	// Handle any errors that occurred during provider creation
	if err != nil {
		return fmt.Errorf("failed to prepare provider: %v", err)
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
