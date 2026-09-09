// Package offload provides reusable helpers for creating and managing
// offload-plugin Kubernetes Secrets.  The functions are decoupled from any
// particular command-options struct so that both the "mapping" and "storage"
// packages can share the same logic.
package offload

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

// SecretOptions holds the fields needed to build an offload Secret.
type SecretOptions struct {
	DefaultOffloadSecret string
	VSphereUsername      string
	VSpherePassword      string
	VSphereURL           string
	StorageUsername      string
	StoragePassword      string
	StorageEndpoint      string
	CACert               string
	InsecureSkipTLS      bool
}

// BuildSecret constructs the offload Secret object without persisting it.
// For dry-run a deterministic Name is used; for live create GenerateName is used.
func BuildSecret(namespace, baseName string, opts SecretOptions, dryRun bool) (*corev1.Secret, error) {
	// Process CA certificate file if specified with @filename
	cacert := opts.CACert
	if strings.HasPrefix(cacert, "@") {
		filePath := cacert[1:]
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate file %s: %v", filePath, err)
		}
		cacert = string(fileContent)
	}

	secretData := map[string][]byte{}

	if opts.StorageUsername != "" {
		secretData["STORAGE_USERNAME"] = []byte(opts.StorageUsername)
	}
	if opts.StoragePassword != "" {
		secretData["STORAGE_PASSWORD"] = []byte(opts.StoragePassword)
	}
	if opts.StorageEndpoint != "" {
		secretData["STORAGE_HOSTNAME"] = []byte(opts.StorageEndpoint)
	}
	if opts.InsecureSkipTLS {
		secretData["STORAGE_SKIP_SSL_VERIFICATION"] = []byte("true")
	}
	if cacert != "" {
		secretData["ca.crt"] = []byte(cacert)
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

// CreateSecret creates an offload Secret on the cluster and returns it.
func CreateSecret(configFlags *genericclioptions.ConfigFlags, namespace, baseName string, opts SecretOptions) (*corev1.Secret, error) {
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	secret, err := BuildSecret(namespace, baseName, opts, false)
	if err != nil {
		return nil, err
	}

	return k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
}

// ValidateSecretFields validates that when any offload-secret fields are
// provided, all required fields are present.
func ValidateSecretFields(opts SecretOptions) error {
	hasInlineFields := opts.VSphereUsername != "" ||
		opts.VSpherePassword != "" ||
		opts.VSphereURL != "" ||
		opts.StorageUsername != "" ||
		opts.StoragePassword != "" ||
		opts.StorageEndpoint != "" ||
		opts.CACert != "" ||
		opts.InsecureSkipTLS

	// Conflict: user supplied both an existing secret name and inline credentials.
	if opts.DefaultOffloadSecret != "" && hasInlineFields {
		return fmt.Errorf("cannot specify both --default-offload-secret and inline offload credentials (--offload-vsphere-username, etc.); use one or the other")
	}

	if !hasInlineFields {
		return nil // No validation needed if no fields provided
	}

	// If any offload fields are provided, validate the required storage credentials.
	// Forklift reads vSphere credentials from the source provider secret rather than
	// from the StorageMap offload secret.
	var missingFields []string

	if opts.StorageUsername == "" {
		missingFields = append(missingFields, "--offload-storage-username")
	}
	if opts.StoragePassword == "" {
		missingFields = append(missingFields, "--offload-storage-password")
	}
	if opts.StorageEndpoint == "" {
		missingFields = append(missingFields, "--offload-storage-endpoint")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("when creating offload secrets inline, all required fields must be provided. Missing: %s", strings.Join(missingFields, ", "))
	}

	return nil
}

// NeedsSecret returns true when the caller should create a new offload secret
// (no existing secret name provided AND at least one credential field is set).
func NeedsSecret(opts SecretOptions) bool {
	return opts.DefaultOffloadSecret == "" &&
		(opts.VSphereUsername != "" ||
			opts.VSpherePassword != "" ||
			opts.VSphereURL != "" ||
			opts.StorageUsername != "" ||
			opts.StoragePassword != "" ||
			opts.StorageEndpoint != "" ||
			opts.CACert != "" ||
			opts.InsecureSkipTLS)
}

// CleanupSecret removes a previously created offload secret (typically on
// rollback after a downstream failure).
func CleanupSecret(configFlags *genericclioptions.ConfigFlags, namespace, secretName string) error {
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get kubernetes client: %v", err)
	}

	return k8sClient.CoreV1().Secrets(namespace).Delete(context.Background(), secretName, metav1.DeleteOptions{})
}
