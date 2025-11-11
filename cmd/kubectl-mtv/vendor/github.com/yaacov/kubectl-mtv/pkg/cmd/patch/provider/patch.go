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

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// PatchProvider patches an existing provider
func PatchProvider(configFlags *genericclioptions.ConfigFlags, name, namespace string,
	url, username, password, cacert string, insecureSkipTLS bool, vddkInitImage, token string,
	domainName, projectName, regionName string, useVddkAioOptimization bool, vddkBufSizeIn64K, vddkBufCount int,
	insecureSkipTLSChanged, useVddkAioOptimizationChanged bool) error {

	klog.V(2).Infof("Patching provider '%s' in namespace '%s'", name, namespace)

	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the existing provider
	existingProvider, err := dynamicClient.Resource(client.ProvidersGVR).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get provider '%s': %v", name, err)
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

	// Track if we need to update credentials
	needsCredentialUpdate := username != "" || password != "" || token != "" || cacert != "" ||
		domainName != "" || projectName != "" || regionName != ""

	// Get and validate secret ownership if credentials need updating
	var secret *corev1.Secret
	if needsCredentialUpdate {
		secret, err = getAndValidateSecret(configFlags, existingProvider)
		if err != nil {
			return err
		}
	}

	// Create a working copy of the spec to build the patch
	patchSpec := make(map[string]interface{})
	providerUpdated := false

	// Update URL if provided
	if url != "" {
		currentURL, _, _ := unstructured.NestedString(existingProvider.Object, "spec", "url")
		klog.V(2).Infof("Updating provider URL from '%s' to '%s'", currentURL, url)
		patchSpec["url"] = url
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

	// Update insecureSkipTLS setting
	if insecureSkipTLSChanged {
		currentSettings["insecureSkipVerify"] = fmt.Sprintf("%t", insecureSkipTLS)
		klog.V(2).Infof("Updated insecureSkipTLS setting to %t", insecureSkipTLS)
		providerUpdated = true
	}

	// Update VDDK settings for vSphere providers
	if providerType == "vsphere" {
		if vddkInitImage != "" {
			klog.V(2).Infof("Updating VDDK init image to '%s'", vddkInitImage)
			currentSettings["vddkInitImage"] = vddkInitImage
			providerUpdated = true
		}

		if useVddkAioOptimizationChanged {
			currentSettings["useVddkAioOptimization"] = fmt.Sprintf("%t", useVddkAioOptimization)
			klog.V(2).Infof("Updated VDDK AIO optimization to %t", useVddkAioOptimization)
			providerUpdated = true
		}

		// Update VDDK configuration if buffer settings are provided
		if vddkBufSizeIn64K > 0 || vddkBufCount > 0 {
			// Get existing vddkConfig or create new one
			existingConfig := currentSettings["vddkConfig"]
			updatedConfig := updateVddkConfig(existingConfig, vddkBufSizeIn64K, vddkBufCount)

			currentSettings["vddkConfig"] = updatedConfig
			klog.V(2).Infof("Updated VDDK configuration: %s", updatedConfig)
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
		secretUpdated, err = updateSecretCredentials(configFlags, secret, providerType,
			username, password, cacert, token, domainName, projectName, regionName)
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
		_, err = dynamicClient.Resource(client.ProvidersGVR).Namespace(namespace).Patch(
			context.TODO(),
			name,
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
		fmt.Printf("provider/%s patched\n", name)
		if secretUpdated {
			klog.V(2).Infof("Updated credentials for provider '%s'", name)
		}
	} else {
		fmt.Printf("provider/%s unchanged (no updates specified)\n", name)
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
	username, password, cacert, token, domainName, projectName, regionName string) (bool, error) {

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
	}

	// Update CA certificate for all types (if applicable)
	if cacert != "" {
		secret.Data["cacert"] = []byte(cacert)
		klog.V(2).Infof("Updated CA certificate")
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
