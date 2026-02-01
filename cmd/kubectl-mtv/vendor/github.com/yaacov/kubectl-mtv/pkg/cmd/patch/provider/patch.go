package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/ec2"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// PatchProviderOptions contains the options for patching a provider
type PatchProviderOptions struct {
	ConfigFlags *genericclioptions.ConfigFlags
	Name        string
	Namespace   string

	// Credentials
	URL      string
	Username string
	Password string
	CACert   string
	Token    string

	// Flags
	InsecureSkipTLS        bool
	InsecureSkipTLSChanged bool

	// vSphere VDDK settings
	VddkInitImage                 string
	UseVddkAioOptimization        bool
	UseVddkAioOptimizationChanged bool
	VddkBufSizeIn64K              int
	VddkBufCount                  int

	// OpenStack settings
	DomainName  string
	ProjectName string
	RegionName  string

	// EC2 settings
	EC2Region             string
	EC2TargetRegion       string
	EC2TargetAZ           string
	EC2TargetAccessKeyID  string
	EC2TargetSecretKey    string
	AutoTargetCredentials bool
}

// PatchProvider patches an existing provider
func PatchProvider(opts PatchProviderOptions) error {
	klog.V(2).Infof("Patching provider '%s' in namespace '%s'", opts.Name, opts.Namespace)

	dynamicClient, err := client.GetDynamicClient(opts.ConfigFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the existing provider
	existingProvider, err := dynamicClient.Resource(client.ProvidersGVR).Namespace(opts.Namespace).Get(context.TODO(), opts.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get provider '%s': %v", opts.Name, err)
	}

	// Get provider type using unstructured operations
	providerType, found, err := unstructured.NestedString(existingProvider.Object, "spec", "type")
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}
	if !found {
		return fmt.Errorf("provider type not found in spec")
	}

	klog.V(3).Infof("Current provider type: %s", providerType)

	// For EC2 provider, use regionName (from --provider-region-name) if ec2Region is empty
	// This allows using --provider-region-name for EC2 regions as shown in documentation
	if providerType == "ec2" && opts.EC2Region == "" && opts.RegionName != "" {
		opts.EC2Region = opts.RegionName
	}

	// Auto-fetch target credentials and target-az from cluster if requested (EC2 only)
	if providerType == "ec2" && opts.AutoTargetCredentials {
		if err := ec2.AutoPopulateTargetOptions(opts.ConfigFlags, &opts.EC2TargetAccessKeyID, &opts.EC2TargetSecretKey, &opts.EC2TargetAZ, &opts.EC2TargetRegion); err != nil {
			return err
		}
	}

	// Track if we need to update credentials
	// Note: AutoTargetCredentials for EC2 providers will populate EC2TargetAccessKeyID and EC2TargetSecretKey above
	needsCredentialUpdate := opts.Username != "" || opts.Password != "" || opts.Token != "" || opts.CACert != "" ||
		opts.DomainName != "" || opts.ProjectName != "" || opts.RegionName != "" || opts.EC2Region != "" ||
		opts.EC2TargetAccessKeyID != "" || opts.EC2TargetSecretKey != "" || opts.InsecureSkipTLSChanged

	// Get and validate secret ownership if credentials need updating
	var secret *corev1.Secret
	if needsCredentialUpdate {
		secret, err = getAndValidateSecret(opts.ConfigFlags, existingProvider)
		if err != nil {
			return err
		}
	}

	// Create a working copy of the spec to build the patch
	patchSpec := make(map[string]interface{})
	providerUpdated := false

	// Update URL if provided
	if opts.URL != "" {
		currentURL, _, _ := unstructured.NestedString(existingProvider.Object, "spec", "url")
		klog.V(2).Infof("Updating provider URL from '%s' to '%s'", currentURL, opts.URL)
		patchSpec["url"] = opts.URL
		providerUpdated = true
	}

	// Get current settings or create empty map for patch
	currentSettings := make(map[string]string)
	existingSettings, found, err := unstructured.NestedStringMap(existingProvider.Object, "spec", "settings")
	if err != nil {
		return fmt.Errorf("failed to get provider settings: %v", err)
	}
	if found && existingSettings != nil {
		for k, v := range existingSettings {
			currentSettings[k] = v
		}
	}

	// Update VDDK settings for vSphere providers
	if providerType == "vsphere" {
		if opts.VddkInitImage != "" {
			klog.V(2).Infof("Updating VDDK init image to '%s'", opts.VddkInitImage)
			currentSettings["vddkInitImage"] = opts.VddkInitImage
			providerUpdated = true
		}

		if opts.UseVddkAioOptimizationChanged {
			currentSettings["useVddkAioOptimization"] = fmt.Sprintf("%t", opts.UseVddkAioOptimization)
			klog.V(2).Infof("Updated VDDK AIO optimization to %t", opts.UseVddkAioOptimization)
			providerUpdated = true
		}

		// Update VDDK configuration if buffer settings are provided
		if opts.VddkBufSizeIn64K > 0 || opts.VddkBufCount > 0 {
			// Get existing vddkConfig or create new one
			existingConfig := currentSettings["vddkConfig"]
			updatedConfig := updateVddkConfig(existingConfig, opts.VddkBufSizeIn64K, opts.VddkBufCount)

			currentSettings["vddkConfig"] = updatedConfig
			klog.V(2).Infof("Updated VDDK configuration: %s", updatedConfig)
			providerUpdated = true
		}
	}

	// Update EC2 settings for EC2 providers
	if providerType == "ec2" {
		if opts.EC2TargetRegion != "" {
			klog.V(2).Infof("Updating EC2 target-region to '%s'", opts.EC2TargetRegion)
			currentSettings["target-region"] = opts.EC2TargetRegion
			providerUpdated = true

			// If no explicit target AZ is provided in this patch, apply the documented default "<target-region>a"
			if opts.EC2TargetAZ == "" {
				defaultAZ := opts.EC2TargetRegion + "a"
				klog.V(2).Infof("No EC2 target-az provided, defaulting to '%s'", defaultAZ)
				currentSettings["target-az"] = defaultAZ
				providerUpdated = true
			}
		}

		if opts.EC2TargetAZ != "" {
			klog.V(2).Infof("Updating EC2 target-az to '%s'", opts.EC2TargetAZ)
			currentSettings["target-az"] = opts.EC2TargetAZ
			providerUpdated = true
		}
	}

	// Add settings to patch if any were modified
	if providerUpdated && len(currentSettings) > 0 {
		patchSpec["settings"] = currentSettings
	}

	// Update credentials if provided and secret is owned by provider
	secretUpdated := false
	if needsCredentialUpdate && secret != nil {
		secretUpdated, err = updateSecretCredentials(opts.ConfigFlags, secret, providerType,
			opts.Username, opts.Password, opts.CACert, opts.Token, opts.DomainName, opts.ProjectName, opts.RegionName,
			opts.EC2Region, opts.EC2TargetAccessKeyID, opts.EC2TargetSecretKey,
			opts.InsecureSkipTLS, opts.InsecureSkipTLSChanged)
		if err != nil {
			return fmt.Errorf("failed to update credentials: %v", err)
		}
	}

	// Apply the patch if any changes were made
	if providerUpdated {
		// Patch the changed spec fields
		patchData := map[string]interface{}{
			"spec": patchSpec,
		}

		patchBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, &unstructured.Unstructured{Object: patchData})
		if err != nil {
			return fmt.Errorf("failed to encode patch data: %v", err)
		}

		// Apply the patch
		_, err = dynamicClient.Resource(client.ProvidersGVR).Namespace(opts.Namespace).Patch(
			context.TODO(),
			opts.Name,
			types.MergePatchType,
			patchBytes,
			metav1.PatchOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to patch provider: %v", err)
		}
	}

	// Provide user feedback
	if providerUpdated || secretUpdated {
		fmt.Printf("provider/%s patched\n", opts.Name)
		if secretUpdated {
			klog.V(2).Infof("Updated credentials for provider '%s'", opts.Name)
		}
	} else {
		fmt.Printf("provider/%s unchanged (no updates specified)\n", opts.Name)
	}

	return nil
}

// getAndValidateSecret retrieves the secret and validates that it's owned by the provider
func getAndValidateSecret(configFlags *genericclioptions.ConfigFlags, provider *unstructured.Unstructured) (*corev1.Secret, error) {
	// Get secret reference using unstructured operations
	secretRef, found, err := unstructured.NestedMap(provider.Object, "spec", "secret")
	if err != nil {
		return nil, fmt.Errorf("failed to get secret reference: %v", err)
	}
	if !found || secretRef == nil {
		return nil, fmt.Errorf("provider has no associated secret")
	}

	secretName, nameOk := secretRef["name"].(string)
	secretNamespace, namespaceOk := secretRef["namespace"].(string)

	if !nameOk || secretName == "" {
		return nil, fmt.Errorf("provider has no associated secret")
	}

	if !namespaceOk || secretNamespace == "" {
		// Use provider namespace if secret namespace is not specified
		secretNamespace = provider.GetNamespace()
	}

	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes client: %v", err)
	}

	// Get the secret
	secret, err := k8sClient.CoreV1().Secrets(secretNamespace).Get(
		context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret '%s': %v", secretName, err)
	}

	// Check if the secret is owned by this provider
	isOwned := false
	for _, ownerRef := range secret.GetOwnerReferences() {
		if ownerRef.Kind == "Provider" && ownerRef.Name == provider.GetName() && ownerRef.UID == provider.GetUID() {
			isOwned = true
			break
		}
	}

	if !isOwned {
		return nil, fmt.Errorf("cannot update credentials: the secret '%s' is not owned by provider '%s'. "+
			"This usually means the secret was created independently and is shared by multiple providers. "+
			"To update credentials, either:\n"+
			"1. Update the secret directly: kubectl patch secret %s -p '{...}'\n"+
			"2. Create a new secret and update the provider to use it: kubectl patch provider %s --secret new-secret-name",
			secret.Name, provider.GetName(), secret.Name, provider.GetName())
	}

	klog.V(2).Infof("Secret '%s' is owned by provider '%s', credentials can be updated", secret.Name, provider.GetName())
	return secret, nil
}

// updateSecretCredentials updates the secret with new credential values
func updateSecretCredentials(configFlags *genericclioptions.ConfigFlags, secret *corev1.Secret, providerType string,
	username, password, cacert, token, domainName, projectName, regionName, ec2Region, ec2TargetAccessKeyID, ec2TargetSecretKey string,
	insecureSkipTLS, insecureSkipTLSChanged bool) (bool, error) {

	updated := false

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}

	// Update credentials based on provider type

	switch providerType {
	case "openshift":
		if token != "" {
			secret.Data["token"] = []byte(token)
			klog.V(2).Infof("Updated OpenShift token")
			updated = true
		}
	case "vsphere", "ovirt", "ova":
		if username != "" {
			secret.Data["user"] = []byte(username)
			klog.V(2).Infof("Updated username")
			updated = true
		}
		if password != "" {
			secret.Data["password"] = []byte(password)
			klog.V(2).Infof("Updated password")
			updated = true
		}
	case "openstack":
		if username != "" {
			secret.Data["username"] = []byte(username)
			klog.V(2).Infof("Updated OpenStack username")
			updated = true
		}
		if password != "" {
			secret.Data["password"] = []byte(password)
			klog.V(2).Infof("Updated OpenStack password")
			updated = true
		}
		if domainName != "" {
			secret.Data["domainName"] = []byte(domainName)
			klog.V(2).Infof("Updated OpenStack domain name")
			updated = true
		}
		if projectName != "" {
			secret.Data["projectName"] = []byte(projectName)
			klog.V(2).Infof("Updated OpenStack project name")
			updated = true
		}
		if regionName != "" {
			secret.Data["regionName"] = []byte(regionName)
			klog.V(2).Infof("Updated OpenStack region name")
			updated = true
		}
	case "ec2":
		if username != "" {
			secret.Data["accessKeyId"] = []byte(username)
			klog.V(2).Infof("Updated EC2 access key ID")
			updated = true
		}
		if password != "" {
			secret.Data["secretAccessKey"] = []byte(password)
			klog.V(2).Infof("Updated EC2 secret access key")
			updated = true
		}
		if ec2Region != "" {
			secret.Data["region"] = []byte(ec2Region)
			klog.V(2).Infof("Updated EC2 region")
			updated = true
		}
		if ec2TargetAccessKeyID != "" {
			secret.Data["targetAccessKeyId"] = []byte(ec2TargetAccessKeyID)
			klog.V(2).Infof("Updated EC2 target account access key ID (cross-account)")
			updated = true
		}
		if ec2TargetSecretKey != "" {
			secret.Data["targetSecretAccessKey"] = []byte(ec2TargetSecretKey)
			klog.V(2).Infof("Updated EC2 target account secret access key (cross-account)")
			updated = true
		}
	}

	// Update CA certificate for all types (if applicable)
	if cacert != "" {
		secret.Data["cacert"] = []byte(cacert)
		klog.V(2).Infof("Updated CA certificate")
		updated = true
	}

	// Update insecureSkipVerify for all types (if changed)
	if insecureSkipTLSChanged {
		if insecureSkipTLS {
			secret.Data["insecureSkipVerify"] = []byte("true")
		} else {
			// Remove the key if insecureSkipTLS is false
			delete(secret.Data, "insecureSkipVerify")
		}
		klog.V(2).Infof("Updated insecureSkipVerify to %t", insecureSkipTLS)
		updated = true
	}

	// Update the secret if any changes were made
	if updated {
		k8sClient, err := client.GetKubernetesClientset(configFlags)
		if err != nil {
			return false, fmt.Errorf("failed to get kubernetes client: %v", err)
		}

		_, err = k8sClient.CoreV1().Secrets(secret.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to update secret: %v", err)
		}
	}

	return updated, nil
}

// updateVddkConfig updates the VDDK configuration block with new buffer settings
func updateVddkConfig(existingConfig string, bufSizeIn64K, bufCount int) string {
	var configBuilder strings.Builder

	// Start with YAML literal block scalar format
	configBuilder.WriteString("|")

	// Parse existing config to preserve other settings
	existingLines := make(map[string]string)
	if existingConfig != "" {
		// Remove the "|" prefix and split by lines
		configContent := strings.TrimPrefix(existingConfig, "|")
		lines := strings.Split(configContent, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Parse key=value pairs
			if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
				existingLines[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	// Update buffer size if provided
	if bufSizeIn64K > 0 {
		existingLines["VixDiskLib.nfcAio.Session.BufSizeIn64K"] = strconv.Itoa(bufSizeIn64K)
	}

	// Update buffer count if provided
	if bufCount > 0 {
		existingLines["VixDiskLib.nfcAio.Session.BufCount"] = strconv.Itoa(bufCount)
	}

	// Build the config string
	for key, value := range existingLines {
		configBuilder.WriteString("\n")
		configBuilder.WriteString(key)
		configBuilder.WriteString("=")
		configBuilder.WriteString(value)
	}

	return configBuilder.String()
}
