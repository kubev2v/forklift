package provider

import (
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/generic"
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
	domainName, projectName, regionName string, useVddkAioOptimization bool, vddkBufSizeIn64K, vddkBufCount int) error {
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
	case "openshift":
		providerResource, secretResource, err = openshift.CreateProvider(configFlags, options)
	case "ovirt":
		providerResource, secretResource, err = generic.CreateProvider(configFlags, options, "ovirt")
	case "openstack":
		providerResource, secretResource, err = openstack.CreateProvider(configFlags, options)
	default:
		// If the provider type is not recognized, return an error
		return fmt.Errorf("unsupported provider type: %s", providerType)
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
