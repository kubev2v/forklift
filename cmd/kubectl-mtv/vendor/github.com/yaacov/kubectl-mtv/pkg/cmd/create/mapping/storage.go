package mapping

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// validateVolumeMode validates volume mode values
func validateVolumeMode(mode string) error {
	switch mode {
	case "Filesystem", "Block":
		return nil
	default:
		return fmt.Errorf("must be one of: Filesystem, Block")
	}
}

// validateAccessMode validates access mode values
func validateAccessMode(mode string) error {
	switch mode {
	case "ReadWriteOnce", "ReadWriteMany", "ReadOnlyMany":
		return nil
	default:
		return fmt.Errorf("must be one of: ReadWriteOnce, ReadWriteMany, ReadOnlyMany")
	}
}

// validateOffloadPlugin validates offload plugin values
func validateOffloadPlugin(plugin string) error {
	switch plugin {
	case "vsphere":
		return nil
	default:
		return fmt.Errorf("must be one of: vsphere")
	}
}

// validateOffloadVendor validates offload vendor values
func validateOffloadVendor(vendor string) error {
	switch vendor {
	case "flashsystem", "vantara", "ontap", "primera3par", "pureFlashArray", "powerflex", "powermax", "powerstore", "infinibox":
		return nil
	default:
		return fmt.Errorf("must be one of: flashsystem, vantara, ontap, primera3par, pureFlashArray, powerflex, powermax, powerstore, infinibox")
	}
}

// StoragePairOptions holds options for parsing storage pairs
type StoragePairOptions struct {
	DefaultVolumeMode    string
	DefaultAccessMode    string
	DefaultOffloadPlugin string
	DefaultOffloadSecret string
	DefaultOffloadVendor string
}

// parseStoragePairsWithOptions parses storage pairs with additional options for VolumeMode, AccessMode, and OffloadPlugin
func parseStoragePairsWithOptions(ctx context.Context, pairStr, defaultNamespace string, configFlags *genericclioptions.ConfigFlags, sourceProvider, inventoryURL string, defaultVolumeMode, defaultAccessMode, defaultOffloadPlugin, defaultOffloadSecret, defaultOffloadVendor string, insecureSkipTLS bool) ([]forkliftv1beta1.StoragePair, error) {
	options := StoragePairOptions{
		DefaultVolumeMode:    defaultVolumeMode,
		DefaultAccessMode:    defaultAccessMode,
		DefaultOffloadPlugin: defaultOffloadPlugin,
		DefaultOffloadSecret: defaultOffloadSecret,
		DefaultOffloadVendor: defaultOffloadVendor,
	}

	return parseStoragePairsInternal(pairStr, defaultNamespace, configFlags, sourceProvider, inventoryURL, &options, insecureSkipTLS)
}

// parseStoragePairsInternal is the internal implementation that handles the parsing logic
func parseStoragePairsInternal(pairStr, defaultNamespace string, configFlags *genericclioptions.ConfigFlags, sourceProvider, inventoryURL string, options *StoragePairOptions, insecureSkipTLS bool) ([]forkliftv1beta1.StoragePair, error) {
	if pairStr == "" {
		return nil, nil
	}

	var pairs []forkliftv1beta1.StoragePair
	pairList := strings.Split(pairStr, ",")

	for _, pairStr := range pairList {
		pairStr = strings.TrimSpace(pairStr)
		if pairStr == "" {
			continue
		}

		// Parse the enhanced format: "source:storage-class;volumeMode=Block;accessMode=ReadWriteOnce;offloadPlugin=vsphere;offloadSecret=secret-name;offloadVendor=vantara"
		pairParts := strings.Split(pairStr, ";")
		if len(pairParts) == 0 {
			continue
		}

		// First part should be source:storage-class
		mainPart := strings.TrimSpace(pairParts[0])
		parts := strings.SplitN(mainPart, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid storage pair format '%s': expected 'source:storage-class' or 'source:namespace/storage-class'", mainPart)
		}

		sourceName := strings.TrimSpace(parts[0])
		targetPart := strings.TrimSpace(parts[1])

		// Parse target part which can be namespace/storage-class or just storage-class
		// Note: namespace is ignored since storage classes are cluster-scoped
		var targetStorageClass string
		if strings.Contains(targetPart, "/") {
			targetParts := strings.SplitN(targetPart, "/", 2)
			// Ignore the namespace part for storage classes since they are cluster-scoped
			targetStorageClass = strings.TrimSpace(targetParts[1])
		} else {
			// Use the target part as storage class
			targetStorageClass = targetPart
		}

		if targetStorageClass == "" {
			return nil, fmt.Errorf("invalid target format '%s': storage class must be specified", targetPart)
		}

		// Parse additional options from remaining parts
		volumeMode := options.DefaultVolumeMode
		accessMode := options.DefaultAccessMode
		offloadPlugin := options.DefaultOffloadPlugin
		offloadSecret := options.DefaultOffloadSecret
		offloadVendor := options.DefaultOffloadVendor

		for i := 1; i < len(pairParts); i++ {
			optionPart := strings.TrimSpace(pairParts[i])
			if optionPart == "" {
				continue
			}

			optionParts := strings.SplitN(optionPart, "=", 2)
			if len(optionParts) != 2 {
				return nil, fmt.Errorf("invalid option format '%s': expected 'key=value'", optionPart)
			}

			key := strings.TrimSpace(optionParts[0])
			value := strings.TrimSpace(optionParts[1])

			switch key {
			case "volumeMode":
				if err := validateVolumeMode(value); err != nil {
					return nil, fmt.Errorf("invalid volumeMode '%s': %v", value, err)
				}
				volumeMode = value
			case "accessMode":
				if err := validateAccessMode(value); err != nil {
					return nil, fmt.Errorf("invalid accessMode '%s': %v", value, err)
				}
				accessMode = value
			case "offloadPlugin":
				if err := validateOffloadPlugin(value); err != nil {
					return nil, fmt.Errorf("invalid offloadPlugin '%s': %v", value, err)
				}
				offloadPlugin = value
			case "offloadSecret":
				offloadSecret = value
			case "offloadVendor":
				if err := validateOffloadVendor(value); err != nil {
					return nil, fmt.Errorf("invalid offloadVendor '%s': %v", value, err)
				}
				offloadVendor = value
			default:
				return nil, fmt.Errorf("unknown option '%s' in storage pair", key)
			}
		}

		// Validate offload configuration completeness
		if (offloadPlugin != "" && offloadVendor == "") || (offloadPlugin == "" && offloadVendor != "") {
			return nil, fmt.Errorf("both offloadPlugin and offloadVendor must be specified together for storage pair '%s'", sourceName)
		}

		// Resolve source storage name to ID
		sourceStorageRefs, err := resolveStorageNameToID(context.TODO(), configFlags, sourceProvider, defaultNamespace, inventoryURL, sourceName, insecureSkipTLS)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve source storage '%s': %v", sourceName, err)
		}

		// Create a pair for each matching source storage resource
		for _, sourceStorageRef := range sourceStorageRefs {
			destination := forkliftv1beta1.DestinationStorage{
				StorageClass: targetStorageClass,
			}

			// Set volume mode if specified
			if volumeMode != "" {
				destination.VolumeMode = corev1.PersistentVolumeMode(volumeMode)
			}

			// Set access mode if specified
			if accessMode != "" {
				destination.AccessMode = corev1.PersistentVolumeAccessMode(accessMode)
			}

			pair := forkliftv1beta1.StoragePair{
				Source:      sourceStorageRef,
				Destination: destination,
			}

			// Set offload plugin if specified
			if offloadPlugin != "" && offloadVendor != "" {
				offloadPluginConfig := &forkliftv1beta1.OffloadPlugin{}

				switch offloadPlugin {
				case "vsphere":
					offloadPluginConfig.VSphereXcopyPluginConfig = &forkliftv1beta1.VSphereXcopyPluginConfig{
						SecretRef:            offloadSecret,
						StorageVendorProduct: forkliftv1beta1.StorageVendorProduct(offloadVendor),
					}
				default:
					return nil, fmt.Errorf("unknown offload plugin '%s' for storage pair '%s': supported plugins are: vsphere", offloadPlugin, sourceName)
				}

				pair.OffloadPlugin = offloadPluginConfig
			}

			pairs = append(pairs, pair)
		}
	}

	return pairs, nil
}

// createStorageMappingWithOptions creates a new storage mapping with additional options for VolumeMode, AccessMode, and OffloadPlugin
func createStorageMappingWithOptions(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace, sourceProvider, targetProvider, storagePairs, inventoryURL string, defaultVolumeMode, defaultAccessMode, defaultOffloadPlugin, defaultOffloadSecret, defaultOffloadVendor string, insecureSkipTLS bool) error {
	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Parse provider references to extract names and namespaces
	sourceProviderName, sourceProviderNamespace := parseProviderReference(sourceProvider, namespace)
	targetProviderName, targetProviderNamespace := parseProviderReference(targetProvider, namespace)

	// Parse storage pairs if provided
	var mappingPairs []forkliftv1beta1.StoragePair
	if storagePairs != "" {
		mappingPairs, err = parseStoragePairsWithOptions(ctx, storagePairs, namespace, configFlags, sourceProvider, inventoryURL, defaultVolumeMode, defaultAccessMode, defaultOffloadPlugin, defaultOffloadSecret, defaultOffloadVendor, insecureSkipTLS)
		if err != nil {
			return fmt.Errorf("failed to parse storage pairs: %v", err)
		}
	}

	// Create a typed StorageMap
	storageMap := &forkliftv1beta1.StorageMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: forkliftv1beta1.StorageMapSpec{
			Provider: provider.Pair{
				Source: corev1.ObjectReference{
					Name:      sourceProviderName,
					Namespace: sourceProviderNamespace,
				},
				Destination: corev1.ObjectReference{
					Name:      targetProviderName,
					Namespace: targetProviderNamespace,
				},
			},
			Map: mappingPairs,
		},
	}

	// Convert to unstructured
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(storageMap)
	if err != nil {
		return fmt.Errorf("failed to convert to unstructured: %v", err)
	}

	mapping := &unstructured.Unstructured{Object: unstructuredObj}
	mapping.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   client.Group,
		Version: client.Version,
		Kind:    "StorageMap",
	})

	_, err = dynamicClient.Resource(client.StorageMapGVR).Namespace(namespace).Create(context.TODO(), mapping, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create storage mapping: %v", err)
	}

	fmt.Printf("storagemap/%s created\n", name)
	return nil
}

// createStorageMappingWithOptionsAndSecret creates a new storage mapping with offload secret creation support
func createStorageMappingWithOptionsAndSecret(ctx context.Context, opts StorageCreateOptions) error {
	var createdOffloadSecret *corev1.Secret
	var err error

	// Validate offload secret creation fields if provided
	if err := validateOffloadSecretFields(opts); err != nil {
		return err
	}

	// Determine if we need to create an offload secret
	if needsOffloadSecret(opts) {
		fmt.Printf("Creating offload secret for storage mapping '%s'\n", opts.Name)

		createdOffloadSecret, err = createOffloadSecret(opts.ConfigFlags, opts.Namespace, opts.Name, opts)
		if err != nil {
			return fmt.Errorf("failed to create offload secret: %v", err)
		}

		// Use the created secret name as the default offload secret
		opts.DefaultOffloadSecret = createdOffloadSecret.Name
		fmt.Printf("Created offload secret '%s' for storage mapping authentication\n", createdOffloadSecret.Name)
	}

	// Call the original function with the updated options
	err = createStorageMappingWithOptions(ctx, opts.ConfigFlags, opts.Name, opts.Namespace,
		opts.SourceProvider, opts.TargetProvider, opts.StoragePairs, opts.InventoryURL,
		opts.DefaultVolumeMode, opts.DefaultAccessMode, opts.DefaultOffloadPlugin,
		opts.DefaultOffloadSecret, opts.DefaultOffloadVendor, opts.InventoryInsecureSkipTLS)

	if err != nil {
		// Clean up the created secret if mapping creation fails
		if createdOffloadSecret != nil {
			if delErr := cleanupOffloadSecret(opts.ConfigFlags, opts.Namespace, createdOffloadSecret.Name); delErr != nil {
				fmt.Printf("Warning: failed to clean up offload secret '%s': %v\n", createdOffloadSecret.Name, delErr)
			}
		}
		return err
	}

	return nil
}

// cleanupOffloadSecret removes a created offload secret on failure
func cleanupOffloadSecret(configFlags *genericclioptions.ConfigFlags, namespace, secretName string) error {
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get kubernetes client: %v", err)
	}

	return k8sClient.CoreV1().Secrets(namespace).Delete(context.Background(), secretName, metav1.DeleteOptions{})
}
