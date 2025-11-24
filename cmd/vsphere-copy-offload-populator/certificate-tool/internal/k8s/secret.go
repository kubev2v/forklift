package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewPopulatorSecret creates a Kubernetes Secret for passing vSphere and storage credentials.
func NewPopulatorSecret(namespace, storageSkipSSLVerification, storagePassword, storageUser, storageUrl, vspherePassword, vsphereUser, vsphereUrl, secretName string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"VSPHERE_INSECURE":              "true",
			"STORAGE_HOSTNAME":              storageUrl,
			"STORAGE_PASSWORD":              storagePassword,
			"STORAGE_USERNAME":              storageUser,
			"GOVMOMI_HOSTNAME":              vsphereUrl,
			"GOVMOMI_PASSWORD":              vspherePassword,
			"GOVMOMI_USERNAME":              vsphereUser,
			"STORAGE_SKIP_SSL_VERIFICATION": storageSkipSSLVerification,
		},
	}
}
