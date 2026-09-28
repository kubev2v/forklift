package inventory

import (
	"context"
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

type typedInventoryOpts struct {
	providerType string
	emptyNoun    string
	headers      []output.Column
	fetch        func(ctx context.Context, pc *ProviderClient) (interface{}, error)
	transform    func(data interface{}) interface{}
}

func listTypedInventoryWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, outputFormat, query string, watchMode, insecureSkipTLS bool, opts typedInventoryOpts) error {
	sq := watch.NewSafeQuery(query)

	return watch.WrapWithWatchAndQuery(watchMode, outputFormat, func() error {
		return listTypedInventoryOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, sq.Get(), insecureSkipTLS, opts)
	}, watch.DefaultInterval, sq.Set, query)
}

func listTypedInventoryOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, outputFormat, query string, insecureSkipTLS bool, opts typedInventoryOpts) error {
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}
	if providerType != opts.providerType {
		return fmt.Errorf("provider type '%s' does not support %s inventory", providerType, opts.emptyNoun)
	}

	data, err := opts.fetch(ctx, providerClient)
	if err != nil {
		return fmt.Errorf("failed to get %s from provider: %v", opts.emptyNoun, err)
	}
	if opts.transform != nil {
		data = opts.transform(data)
	}

	var queryOpts *querypkg.QueryOptions
	if query != "" {
		queryOpts, err = querypkg.ParseQueryString(query)
		if err != nil {
			return fmt.Errorf("failed to parse query: %v", err)
		}
		data, err = querypkg.ApplyQueryInterface(data, query)
		if err != nil {
			return fmt.Errorf("failed to apply query: %v", err)
		}
	}

	emptyMessage := fmt.Sprintf("No %s found for provider %s", opts.emptyNoun, providerName)
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(data, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(data, emptyMessage)
	case "markdown":
		return output.PrintMarkdownWithQuery(data, opts.headers, queryOpts, emptyMessage)
	case "table":
		return output.PrintTableWithQuery(data, opts.headers, queryOpts, emptyMessage)
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}

// ListVSphereCustomFieldDefsWithInsecure lists vSphere custom field definitions.
func ListVSphereCustomFieldDefsWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, outputFormat, query string, watchMode, insecureSkipTLS bool) error {
	return listTypedInventoryWithInsecure(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, watchMode, insecureSkipTLS, typedInventoryOpts{
		providerType: "vsphere",
		emptyNoun:    "custom field definitions",
		headers: []output.Column{
			{Title: "NAME", Key: "name"},
			{Title: "ID", Key: "id"},
			{Title: "KEY", Key: "key"},
			{Title: "MANAGED-OBJECT-TYPE", Key: "managedObjectType"},
		},
		fetch: func(ctx context.Context, pc *ProviderClient) (interface{}, error) {
			return pc.GetCustomFieldDefs(ctx, 4)
		},
	})
}

// ListOVirtServerCpusWithInsecure lists oVirt server CPU types.
func ListOVirtServerCpusWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, outputFormat, query string, watchMode, insecureSkipTLS bool) error {
	return listTypedInventoryWithInsecure(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, watchMode, insecureSkipTLS, typedInventoryOpts{
		providerType: "ovirt",
		emptyNoun:    "server CPUs",
		headers: []output.Column{
			{Title: "NAME", Key: "name"},
			{Title: "ID", Key: "id"},
			{Title: "PATH", Key: "path"},
		},
		fetch: func(ctx context.Context, pc *ProviderClient) (interface{}, error) {
			return pc.GetServerCpus(ctx, 4)
		},
	})
}

// ListOpenStackRegionsWithInsecure lists OpenStack regions.
func ListOpenStackRegionsWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, outputFormat, query string, watchMode, insecureSkipTLS bool) error {
	return listTypedInventoryWithInsecure(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, watchMode, insecureSkipTLS, typedInventoryOpts{
		providerType: "openstack",
		emptyNoun:    "regions",
		headers: []output.Column{
			{Title: "NAME", Key: "name"},
			{Title: "ID", Key: "id"},
			{Title: "DESCRIPTION", Key: "description"},
			{Title: "PARENT-REGION", Key: "parentRegionID"},
		},
		fetch: func(ctx context.Context, pc *ProviderClient) (interface{}, error) {
			return pc.GetRegions(ctx, 4)
		},
	})
}

// ListOpenShiftInstanceTypesWithInsecure lists OpenShift VM instance types.
func ListOpenShiftInstanceTypesWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, outputFormat, query string, watchMode, insecureSkipTLS bool) error {
	return listTypedInventoryWithInsecure(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, watchMode, insecureSkipTLS, typedInventoryOpts{
		providerType: "openshift",
		emptyNoun:    "instance types",
		headers: []output.Column{
			{Title: "NAME", Key: "name"},
			{Title: "ID", Key: "id"},
			{Title: "NAMESPACE", Key: "namespace"},
		},
		fetch: func(ctx context.Context, pc *ProviderClient) (interface{}, error) {
			return pc.GetInstanceTypes(ctx, 4)
		},
	})
}

// ListOpenShiftClusterInstanceTypesWithInsecure lists OpenShift cluster VM instance types.
func ListOpenShiftClusterInstanceTypesWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, outputFormat, query string, watchMode, insecureSkipTLS bool) error {
	return listTypedInventoryWithInsecure(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, watchMode, insecureSkipTLS, typedInventoryOpts{
		providerType: "openshift",
		emptyNoun:    "cluster instance types",
		headers: []output.Column{
			{Title: "NAME", Key: "name"},
			{Title: "ID", Key: "id"},
		},
		fetch: func(ctx context.Context, pc *ProviderClient) (interface{}, error) {
			return pc.GetClusterInstanceTypes(ctx, 4)
		},
	})
}

// ListOpenShiftKubeVirtsWithInsecure lists OpenShift KubeVirt CRs.
func ListOpenShiftKubeVirtsWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, outputFormat, query string, watchMode, insecureSkipTLS bool) error {
	return listTypedInventoryWithInsecure(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, watchMode, insecureSkipTLS, typedInventoryOpts{
		providerType: "openshift",
		emptyNoun:    "KubeVirt CRs",
		headers: []output.Column{
			{Title: "NAME", Key: "name"},
			{Title: "ID", Key: "id"},
			{Title: "NAMESPACE", Key: "namespace"},
		},
		fetch: func(ctx context.Context, pc *ProviderClient) (interface{}, error) {
			return pc.GetKubeVirts(ctx, 4)
		},
	})
}

// ListNutanixImagesWithInsecure lists Nutanix images.
func ListNutanixImagesWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, outputFormat, query string, watchMode, insecureSkipTLS bool) error {
	return listTypedInventoryWithInsecure(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, watchMode, insecureSkipTLS, typedInventoryOpts{
		providerType: "nutanix",
		emptyNoun:    "images",
		headers: []output.Column{
			{Title: "NAME", Key: "name"},
			{Title: "ID", Key: "id"},
			{Title: "TYPE", Key: "imageType"},
			{Title: "SIZE", Key: "sizeHuman"},
			{Title: "ARCH", Key: "architecture"},
		},
		fetch: func(ctx context.Context, pc *ProviderClient) (interface{}, error) {
			return pc.GetImages(ctx, 4)
		},
		transform: addHumanReadableNutanixImageSizes,
	})
}

func addHumanReadableNutanixImageSizes(data interface{}) interface{} {
	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			if image, ok := item.(map[string]interface{}); ok {
				setNutanixImageSizeHuman(image)
			}
		}
	case map[string]interface{}:
		setNutanixImageSizeHuman(v)
	}
	return data
}

func setNutanixImageSizeHuman(image map[string]interface{}) {
	if size, exists := image["sizeBytes"]; exists {
		if sizeVal, ok := size.(float64); ok {
			image["sizeHuman"] = humanizeBytes(sizeVal)
		}
	}
}
