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

// buildOffloadSecret constructs the offload Secret object without persisting it.
// For dry-run a deterministic Name is used; for live create GenerateName is used.
func buildOffloadSecret(namespace, baseName string, opts StorageCreateOptions, dryRun bool) (*corev1.Secret, error) {
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

	secretData := map[string][]byte{}

	if opts.OffloadVSphereUsername != "" {
		secretData["user"] = []byte(opts.OffloadVSphereUsername)
	}
	if opts.OffloadVSpherePassword != "" {
		secretData["password"] = []byte(opts.OffloadVSpherePassword)
	}
	if opts.OffloadVSphereURL != "" {
		secretData["url"] = []byte(opts.OffloadVSphereURL)
	}
	if opts.OffloadStorageUsername != "" {
		secretData["storageUser"] = []byte(opts.OffloadStorageUsername)
	}
	if opts.OffloadStoragePassword != "" {
		secretData["storagePassword"] = []byte(opts.OffloadStoragePassword)
	}
	if opts.OffloadStorageEndpoint != "" {
		secretData["storageEndpoint"] = []byte(opts.OffloadStorageEndpoint)
	}
	if opts.OffloadInsecureSkipTLS {
		secretData["insecureSkipVerify"] = []byte("true")
	}
	if cacert != "" {
		secretData["cacert"] = []byte(cacert)
	}

	if len(secretData) == 0 {
		return nil, fmt.Errorf("no offload secret fields provided")
	}

	meta := metav1.ObjectMeta{
		Namespace: namespace,
		Labels: map[string]string{
			"createdForResourceType": "offload",
			"createdForMapping":      baseName,
		},
	}
	if dryRun {
		meta.Name = fmt.Sprintf("%s-offload", baseName)
	} else {
		meta.GenerateName = fmt.Sprintf("%s-offload-", baseName)
	}

	return &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: meta,
		Data:       secretData,
		Type:       corev1.SecretTypeOpaque,
	}, nil
}

// createOffloadSecret creates a secret for offload plugin authentication
func createOffloadSecret(configFlags *genericclioptions.ConfigFlags, namespace, baseName string, opts StorageCreateOptions) (*corev1.Secret, error) {
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	secret, err := buildOffloadSecret(namespace, baseName, opts, false)
	if err != nil {
		return nil, err
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
