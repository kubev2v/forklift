package mapping

import (
	"context"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// createOffloadSecret creates a secret for offload plugin authentication
func createOffloadSecret(configFlags *genericclioptions.ConfigFlags, namespace, baseName string, opts StorageCreateOptions) (*corev1.Secret, error) {
	// Get the Kubernetes client using configFlags
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	// Process CA certificate file if specified with @filename
	cacert := opts.OffloadCACert
	if strings.HasPrefix(cacert, "@") {
		filePath := cacert[1:]
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate file %s: %v", filePath, err)
		}
		cacert = string(fileContent)
	}

	// Create secret data without base64 encoding (the API handles this automatically)
	secretData := map[string][]byte{}

	// Add vSphere credentials (required)
	if opts.OffloadVSphereUsername != "" {
		secretData["user"] = []byte(opts.OffloadVSphereUsername)
	}
	if opts.OffloadVSpherePassword != "" {
		secretData["password"] = []byte(opts.OffloadVSpherePassword)
	}
	if opts.OffloadVSphereURL != "" {
		secretData["url"] = []byte(opts.OffloadVSphereURL)
	}

	// Add storage array credentials (required)
	if opts.OffloadStorageUsername != "" {
		secretData["storageUser"] = []byte(opts.OffloadStorageUsername)
	}
	if opts.OffloadStoragePassword != "" {
		secretData["storagePassword"] = []byte(opts.OffloadStoragePassword)
	}
	if opts.OffloadStorageEndpoint != "" {
		secretData["storageEndpoint"] = []byte(opts.OffloadStorageEndpoint)
	}

	// Add optional TLS fields
	if opts.OffloadInsecureSkipTLS {
		secretData["insecureSkipVerify"] = []byte("true")
	}
	if cacert != "" {
		secretData["cacert"] = []byte(cacert)
	}

	// Validate that we have the minimum required fields
	if len(secretData) == 0 {
		return nil, fmt.Errorf("no offload secret fields provided")
	}

	// Generate a name prefix for the secret
	secretName := fmt.Sprintf("%s-offload-", baseName)

	// Create the secret object directly as a typed Secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: secretName,
			Namespace:    namespace,
			Labels: map[string]string{
				"createdForResourceType": "offload",
				"createdForMapping":      baseName,
			},
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}

	return k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
}

// validateOffloadSecretFields validates that required fields are present for offload secret creation
func validateOffloadSecretFields(opts StorageCreateOptions) error {
	// Check if any offload secret creation fields are provided
	hasOffloadFields := opts.OffloadVSphereUsername != "" ||
		opts.OffloadVSpherePassword != "" ||
		opts.OffloadVSphereURL != "" ||
		opts.OffloadStorageUsername != "" ||
		opts.OffloadStoragePassword != "" ||
		opts.OffloadStorageEndpoint != "" ||
		opts.OffloadCACert != "" ||
		opts.OffloadInsecureSkipTLS

	if !hasOffloadFields {
		return nil // No validation needed if no fields provided
	}

	// If any offload fields are provided, validate required combinations
	var missingFields []string

	// vSphere credentials are required
	if opts.OffloadVSphereUsername == "" {
		missingFields = append(missingFields, "--offload-vsphere-username")
	}
	if opts.OffloadVSpherePassword == "" {
		missingFields = append(missingFields, "--offload-vsphere-password")
	}
	if opts.OffloadVSphereURL == "" {
		missingFields = append(missingFields, "--offload-vsphere-url")
	}

	// Storage credentials are required
	if opts.OffloadStorageUsername == "" {
		missingFields = append(missingFields, "--offload-storage-username")
	}
	if opts.OffloadStoragePassword == "" {
		missingFields = append(missingFields, "--offload-storage-password")
	}
	if opts.OffloadStorageEndpoint == "" {
		missingFields = append(missingFields, "--offload-storage-endpoint")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("when creating offload secrets inline, all required fields must be provided. Missing: %s", strings.Join(missingFields, ", "))
	}

	return nil
}

// needsOffloadSecret determines if we should create an offload secret
func needsOffloadSecret(opts StorageCreateOptions) bool {
	// Only create a secret if:
	// 1. No existing secret name is provided AND
	// 2. Some offload secret creation fields are provided
	return opts.DefaultOffloadSecret == "" &&
		(opts.OffloadVSphereUsername != "" ||
			opts.OffloadVSpherePassword != "" ||
			opts.OffloadVSphereURL != "" ||
			opts.OffloadStorageUsername != "" ||
			opts.OffloadStoragePassword != "" ||
			opts.OffloadStorageEndpoint != "" ||
			opts.OffloadCACert != "" ||
			opts.OffloadInsecureSkipTLS)
}
