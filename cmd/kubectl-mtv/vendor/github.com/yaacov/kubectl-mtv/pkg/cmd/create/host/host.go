package host

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// CreateHostOptions encapsulates the parameters for creating migration hosts.
// This includes provider information, authentication details, network configuration,
// and TLS settings for host connections.
type CreateHostOptions struct {
	HostIDs                  []string
	Namespace                string
	Provider                 string
	ConfigFlags              *genericclioptions.ConfigFlags
	InventoryURL             string
	InventoryInsecureSkipTLS bool
	Username                 string
	Password                 string
	ExistingSecret           string
	IPAddress                string
	NetworkAdapterName       string
	HostInsecureSkipTLS      bool
	CACert                   string
	HostSpec                 forkliftv1beta1.HostSpec
}

// Create creates new migration hosts for vSphere providers.
// It validates the provider, handles authentication (existing secret, provider secret, or new secret),
// resolves IP addresses from network adapters or direct input, creates the host resources,
// and establishes proper ownership relationships between providers, hosts, and secrets.
func Create(ctx context.Context, opts CreateHostOptions) error {
	// Get the provider object and validate it's a vSphere provider
	// Only vSphere providers support host creation
	_, err := validateAndGetProvider(ctx, opts.ConfigFlags, opts.Provider, opts.Namespace)
	if err != nil {
		return err
	}

	// Fetch available hosts from provider inventory to validate requested host names
	// and extract network adapter information for IP resolution
	availableHosts, err := getProviderHosts(ctx, opts.ConfigFlags, opts.Provider, opts.Namespace, opts.InventoryURL, opts.InventoryInsecureSkipTLS)
	if err != nil {
		return fmt.Errorf("failed to get provider hosts: %v", err)
	}

	// Ensure all requested host IDs exist in the provider's inventory
	if err := validateHostIDs(opts.HostIDs, availableHosts); err != nil {
		return err
	}

	// Create or get secret
	var secret *corev1.ObjectReference
	var createdSecret *corev1.Secret

	// Determine authentication strategy: ESXi endpoints can reuse provider secrets,
	// otherwise use existing secret or create new one
	providerHasESXIEndpoint, providerSecret, err := CheckProviderESXIEndpoint(ctx, opts.ConfigFlags, opts.Provider, opts.Namespace)
	if err != nil {
		return fmt.Errorf("failed to check provider endpoint type: %v", err)
	}

	if providerHasESXIEndpoint && opts.ExistingSecret == "" && opts.Username == "" {
		// For ESXi endpoints, reuse the provider's existing secret for efficiency
		secret = providerSecret
		klog.V(2).Infof("Using provider secret '%s' for ESXi endpoint", providerSecret.Name)
	} else if opts.ExistingSecret != "" {
		// Use user-specified existing secret
		secret = &corev1.ObjectReference{
			Name:      opts.ExistingSecret,
			Namespace: opts.Namespace,
		}
	} else {
		// Create new secret with provided credentials
		// Use first host ID for secret naming when creating multiple hosts
		firstHostID := opts.HostIDs[0]
		firstHostResourceName := firstHostID + "-" + generateHash(firstHostID)
		createdSecret, err = createHostSecret(opts.ConfigFlags, opts.Namespace, firstHostResourceName, opts.Username, opts.Password, opts.HostInsecureSkipTLS, opts.CACert)
		if err != nil {
			return fmt.Errorf("failed to create host secret: %v", err)
		}
		secret = &corev1.ObjectReference{
			Name:      createdSecret.Name,
			Namespace: createdSecret.Namespace,
		}
	}

	// Create each host resource with proper ownership and secret references
	for _, hostID := range opts.HostIDs {
		// Resolve IP address from direct input or network adapter lookup
		hostIP, err := resolveHostIPAddress(opts.IPAddress, opts.NetworkAdapterName, hostID, availableHosts)
		if err != nil {
			return fmt.Errorf("failed to resolve IP address for host %s: %v", hostID, err)
		}

		// Create the host resource with provider ownership
		hostObj, err := createSingleHost(ctx, opts.ConfigFlags, opts.Namespace, hostID, opts.Provider, hostIP, secret, availableHosts)
		if err != nil {
			return fmt.Errorf("failed to create host %s: %v", hostID, err)
		}

		// If we created a new secret, add this host as an owner for proper garbage collection
		// This ensures the secret is deleted only when all hosts using it are deleted
		if createdSecret != nil {
			err = addHostAsSecretOwner(opts.ConfigFlags, opts.Namespace, createdSecret.Name, hostObj)
			if err != nil {
				return fmt.Errorf("failed to add host %s as owner of secret %s: %v", hostID, createdSecret.Name, err)
			}
		}

		// Inform user about the created resource
		fmt.Printf("host/%s created\n", hostObj.Name)

		klog.V(2).Infof("Created host '%s' in namespace '%s'", hostID, opts.Namespace)
	}

	if createdSecret != nil {
		klog.V(2).Infof("Created secret '%s' for host authentication", createdSecret.Name)
	}

	return nil
}

// validateAndGetProvider validates that the specified provider exists and is a vSphere provider.
// Only vSphere providers support host creation since hosts represent ESXi servers.
func validateAndGetProvider(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace string) (*unstructured.Unstructured, error) {
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %v", err)
	}

	// Verify this is a vSphere provider - other provider types don't support hosts
	providerType, found, err := unstructured.NestedString(provider.Object, "spec", "type")
	if err != nil || !found {
		return nil, fmt.Errorf("failed to get provider type: %v", err)
	}

	if providerType != "vsphere" {
		return nil, fmt.Errorf("only vSphere providers support host creation, got provider type: %s", providerType)
	}

	return provider, nil
}

// getProviderHosts retrieves the list of available ESXi hosts from the provider's inventory.
// This information is used to validate host names and extract network adapter details.
func getProviderHosts(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]map[string]interface{}, error) {
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, err
	}

	// Create a new provider client
	providerClient := inventory.NewProviderClientWithInsecure(configFlags, provider, inventoryURL, insecureSkipTLS)

	// Fetch hosts inventory
	data, err := providerClient.GetHosts(ctx, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch host inventory: %v", err)
	}

	// Convert to expected format
	dataArray, ok := data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for host inventory")
	}

	hosts := make([]map[string]interface{}, 0, len(dataArray))
	for _, item := range dataArray {
		if host, ok := item.(map[string]interface{}); ok {
			hosts = append(hosts, host)
		}
	}

	return hosts, nil
}

// validateHostIDs ensures all requested host IDs exist in the provider's inventory.
// This prevents creation of host resources that reference non-existent ESXi hosts.
func validateHostIDs(hostIDs []string, availableHosts []map[string]interface{}) error {
	hostMap := make(map[string]bool)
	for _, host := range availableHosts {
		if id, ok := host["id"].(string); ok {
			hostMap[id] = true
		}
	}

	var missingHosts []string
	for _, hostID := range hostIDs {
		if !hostMap[hostID] {
			missingHosts = append(missingHosts, hostID)
		}
	}

	if len(missingHosts) > 0 {
		return fmt.Errorf("the following host IDs were not found in provider inventory: %s", strings.Join(missingHosts, ", "))
	}

	return nil
}

// resolveHostIPAddress determines the IP address to use for host communication.
// It supports either direct IP specification or lookup from a named network adapter
// in the host's inventory data.
func resolveHostIPAddress(directIP, networkAdapterName, hostID string, availableHosts []map[string]interface{}) (string, error) {
	if directIP != "" {
		return directIP, nil
	}

	// Search through host inventory to find the specified network adapter
	for _, host := range availableHosts {
		if id, ok := host["id"].(string); ok && id == hostID {
			if networkAdapters, ok := host["networkAdapters"].([]interface{}); ok {
				for _, adapter := range networkAdapters {
					if adapterMap, ok := adapter.(map[string]interface{}); ok {
						if adapterName, ok := adapterMap["name"].(string); ok && adapterName == networkAdapterName {
							if ipAddress, ok := adapterMap["ipAddress"].(string); ok {
								return ipAddress, nil
							}
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("network adapter '%s' not found for host '%s' or no IP address available", networkAdapterName, hostID)
}

// createSingleHost creates a single Host resource with proper ownership by the provider.
// It extracts the host ID from inventory, sets up owner references, and creates the Kubernetes resource.
// Returns the created host object for use in establishing secret ownership.
func createSingleHost(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace, hostID, providerName, ipAddress string, secret *corev1.ObjectReference, availableHosts []map[string]interface{}) (*forkliftv1beta1.Host, error) {
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider for ownership: %v", err)
	}

	// Create Host resource with provider as controlling owner for lifecycle management
	hostResourceName := hostID + "-" + generateHash(hostID)
	hostObj := &forkliftv1beta1.Host{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostResourceName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: provider.GetAPIVersion(),
					Kind:       provider.GetKind(),
					Name:       provider.GetName(),
					UID:        provider.GetUID(),
					Controller: &[]bool{true}[0], // Provider controls host lifecycle
				},
			},
		},
		Spec: forkliftv1beta1.HostSpec{
			Provider: corev1.ObjectReference{
				Kind:       "Provider",
				APIVersion: forkliftv1beta1.SchemeGroupVersion.String(),
				Name:       providerName,
				Namespace:  namespace,
			},
			IpAddress: ipAddress,
			Secret:    *secret,
		},
	}

	hostObj.Spec.ID = hostID
	hostObj.Spec.Name = getHostNameFromID(hostID, availableHosts)
	hostObj.Kind = "Host"
	hostObj.APIVersion = forkliftv1beta1.SchemeGroupVersion.String()

	unstructuredHost, err := runtime.DefaultUnstructuredConverter.ToUnstructured(hostObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert host to unstructured: %v", err)
	}

	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client: %v", err)
	}

	createdHostUnstructured, err := dynamicClient.Resource(client.HostsGVR).Namespace(namespace).Create(
		context.Background(),
		&unstructured.Unstructured{Object: unstructuredHost},
		metav1.CreateOptions{},
	)

	if err != nil {
		if errors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("host '%s' already exists in namespace '%s'", hostID, namespace)
		}
		return nil, fmt.Errorf("failed to create host: %v", err)
	}

	var createdHost forkliftv1beta1.Host
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(createdHostUnstructured.Object, &createdHost)
	if err != nil {
		return nil, fmt.Errorf("failed to convert created host back to typed object: %v", err)
	}

	return &createdHost, nil
}

// getHostNameFromID returns the host name for a given host ID from inventory
func getHostNameFromID(hostID string, availableHosts []map[string]interface{}) string {
	for _, host := range availableHosts {
		if id, ok := host["id"].(string); ok && id == hostID {
			if name, ok := host["name"].(string); ok {
				return name
			}
		}
	}
	return hostID // fallback to ID if name not found
}

// generateHash creates a 4-letter hash from the input string for collision prevention
func generateHash(input string) string {
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:2]) // 2 bytes = 4 hex characters
}

// createHostSecret creates a Kubernetes Secret containing host authentication credentials.
// The secret is labeled to associate it with the host resource and includes optional
// TLS configuration for secure or insecure connections.
func createHostSecret(configFlags *genericclioptions.ConfigFlags, namespace, hostResourceName, username, password string, hostInsecureSkipTLS bool, cacert string) (*corev1.Secret, error) {
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	secretData := map[string][]byte{
		"user":     []byte(username),
		"password": []byte(password),
	}

	if hostInsecureSkipTLS {
		secretData["insecureSkipVerify"] = []byte("true")
	}
	if cacert != "" {
		secretData["cacert"] = []byte(cacert)
	}

	secretName := fmt.Sprintf("%s-host-", hostResourceName)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: secretName,
			Namespace:    namespace,
			Labels: map[string]string{
				"createdForResource":     hostResourceName,
				"createdForResourceType": "hosts",
			},
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}

	return k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
}

// addHostAsSecretOwner adds a host as an owner reference to a secret, enabling proper
// garbage collection. When multiple hosts share a secret, each becomes an owner, and
// the secret is only deleted when all owning hosts are removed.
func addHostAsSecretOwner(configFlags *genericclioptions.ConfigFlags, namespace, secretName string, host *forkliftv1beta1.Host) error {
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	secret, err := k8sClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret %s: %v", secretName, err)
	}

	// Create owner reference for the host (non-controller since multiple hosts can own the secret)
	hostOwnerRef := metav1.OwnerReference{
		APIVersion: host.APIVersion,
		Kind:       host.Kind,
		Name:       host.Name,
		UID:        host.UID,
		Controller: &[]bool{false}[0], // Multiple hosts can own the same secret
	}

	// Check if this host is already an owner to avoid duplicates
	for _, ownerRef := range secret.OwnerReferences {
		if ownerRef.UID == host.UID {
			return nil // Already an owner, nothing to do
		}
	}

	secret.OwnerReferences = append(secret.OwnerReferences, hostOwnerRef)

	_, err = k8sClient.CoreV1().Secrets(namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret %s with host owner reference: %v", secretName, err)
	}

	return nil
}

// CheckProviderESXIEndpoint determines if a provider is configured with ESXi endpoint type
// and returns its secret reference. ESXi endpoints allow hosts to reuse the provider's
// authentication credentials for efficiency.
func CheckProviderESXIEndpoint(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace string) (bool, *corev1.ObjectReference, error) {
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return false, nil, err
	}

	settings, found, err := unstructured.NestedMap(provider.Object, "spec", "settings")
	if err != nil || !found {
		return false, nil, nil
	}

	sdkEndpoint, ok := settings["sdkEndpoint"].(string)
	if !ok || sdkEndpoint != "esxi" {
		return false, nil, nil
	}

	secretName, found, err := unstructured.NestedString(provider.Object, "spec", "secret", "name")
	if err != nil || !found {
		return false, nil, fmt.Errorf("provider has esxi endpoint but no secret configured")
	}

	secretNamespace, found, err := unstructured.NestedString(provider.Object, "spec", "secret", "namespace")
	if err != nil || !found {
		secretNamespace = namespace
	}

	return true, &corev1.ObjectReference{
		Name:      secretName,
		Namespace: secretNamespace,
	}, nil
}
