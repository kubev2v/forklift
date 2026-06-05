package mapping

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/mapping/offload"
)

// toSecretOptions converts the offload-related fields of StorageCreateOptions
// into the shared offload.SecretOptions.
func toSecretOptions(opts StorageCreateOptions) offload.SecretOptions {
	return offload.SecretOptions{
		DefaultOffloadSecret: opts.DefaultOffloadSecret,
		VSphereUsername:      opts.OffloadVSphereUsername,
		VSpherePassword:      opts.OffloadVSpherePassword,
		VSphereURL:           opts.OffloadVSphereURL,
		StorageUsername:      opts.OffloadStorageUsername,
		StoragePassword:      opts.OffloadStoragePassword,
		StorageEndpoint:      opts.OffloadStorageEndpoint,
		CACert:               opts.OffloadCACert,
		InsecureSkipTLS:      opts.OffloadInsecureSkipTLS,
	}
}

// buildOffloadSecret constructs the offload Secret object without persisting it.
// For dry-run a deterministic Name is used; for live create GenerateName is used.
func buildOffloadSecret(namespace, baseName string, opts StorageCreateOptions, dryRun bool) (*corev1.Secret, error) {
	return offload.BuildSecret(namespace, baseName, toSecretOptions(opts), dryRun)
}

// createOffloadSecret creates a secret for offload plugin authentication
func createOffloadSecret(configFlags *genericclioptions.ConfigFlags, namespace, baseName string, opts StorageCreateOptions) (*corev1.Secret, error) {
	return offload.CreateSecret(configFlags, namespace, baseName, toSecretOptions(opts))
}

// validateOffloadSecretFields validates that required fields are present for offload secret creation
func validateOffloadSecretFields(opts StorageCreateOptions) error {
	return offload.ValidateSecretFields(toSecretOptions(opts))
}

// needsOffloadSecret determines if we should create an offload secret
func needsOffloadSecret(opts StorageCreateOptions) bool {
	return offload.NeedsSecret(toSecretOptions(opts))
}
