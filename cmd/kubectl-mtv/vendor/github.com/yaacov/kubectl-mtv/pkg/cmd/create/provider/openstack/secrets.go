package openstack

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// createSecret creates a secret for OpenStack providers with correct field names
func createSecret(configFlags *genericclioptions.ConfigFlags, namespace, providerName, username, password, url, cacert, token string, insecureSkipTLS bool, domainName, projectName, regionName string) (*corev1.Secret, error) {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %v", err)
	}

	secretName := fmt.Sprintf("%s-openstack-secret", providerName)

	// Prepare secret data
	secretData := map[string][]byte{
		"url": []byte(url),
	}

	// Add authentication data based on what's provided
	if token != "" {
		secretData["token"] = []byte(token)
	} else {
		// Use 'username' instead of 'user' for OpenStack
		secretData["username"] = []byte(username)
		secretData["password"] = []byte(password)
	}

	// Add CA certificate if provided
	if cacert != "" {
		secretData["cacert"] = []byte(cacert)
	}

	// Add insecureSkipVerify if true
	if insecureSkipTLS {
		secretData["insecureSkipVerify"] = []byte("true")
	}

	// Add OpenStack specific fields
	if domainName != "" {
		secretData["domainName"] = []byte(domainName)
	}
	if projectName != "" {
		secretData["projectName"] = []byte(projectName)
	}
	if regionName != "" {
		secretData["regionName"] = []byte(regionName)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"createdForProviderType": "openstack",
				"createdForResourceType": "providers",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	// Convert secret to unstructured
	unstructSecret, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to convert secret to unstructured: %v", err)
	}

	unstructuredSecret := &unstructured.Unstructured{Object: unstructSecret}

	// Create the secret
	createdUnstructSecret, err := c.Resource(client.SecretsGVR).Namespace(namespace).Create(context.TODO(), unstructuredSecret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create secret: %v", err)
	}

	// Convert unstructured secret back to typed secret
	createdSecret := &corev1.Secret{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(createdUnstructSecret.Object, createdSecret); err != nil {
		return nil, fmt.Errorf("failed to convert secret from unstructured: %v", err)
	}

	return createdSecret, nil
}
