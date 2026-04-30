package generic

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

// buildSecret returns a secret for generic providers (oVirt) without submitting it to the API.
func buildSecret(namespace, providerName, user, password, url, cacert, token string, insecureSkipTLS bool, domainName, projectName, regionName, providerType string) *corev1.Secret {
	secretName := fmt.Sprintf("%s-provider-secret", providerName)

	secretData := map[string][]byte{
		"url": []byte(url),
	}

	if token != "" {
		secretData["token"] = []byte(token)
	} else {
		secretData["user"] = []byte(user)
		secretData["password"] = []byte(password)
	}

	if cacert != "" {
		secretData["cacert"] = []byte(cacert)
	}

	if insecureSkipTLS {
		secretData["insecureSkipVerify"] = []byte("true")
	}

	if providerType == "openstack" {
		if domainName != "" {
			secretData["domainName"] = []byte(domainName)
		}
		if projectName != "" {
			secretData["projectName"] = []byte(projectName)
		}
		if regionName != "" {
			secretData["regionName"] = []byte(regionName)
		}
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}
}

// createSecret creates a secret for generic providers (oVirt, OpenStack)
func createSecret(configFlags *genericclioptions.ConfigFlags, namespace, providerName, user, password, url, cacert, token string, insecureSkipTLS bool, domainName, projectName, regionName, providerType string) (*corev1.Secret, error) {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %v", err)
	}

	secret := buildSecret(namespace, providerName, user, password, url, cacert, token, insecureSkipTLS, domainName, projectName, regionName, providerType)

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
