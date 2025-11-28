package client

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// GetServiceAccountTokenForInventory attempts to retrieve a service account token
// for use with the MTV inventory service when the kubeconfig uses client certificate authentication.
//
// This is needed for Kind and similar clusters where:
// - The kubeconfig uses client certificates for authentication
// - The MTV inventory service validates bearer tokens, not client certificates
//
// The function tries to get the forklift-controller service account token which has
// the necessary permissions to access the inventory API.
//
// RBAC Requirements: The service account running this code must have 'get' and 'list'
// permissions on secrets in the konveyor-forklift namespace.
//
// Returns: bearer token string (empty if not found), success boolean
func GetServiceAccountTokenForInventory(ctx context.Context, config *rest.Config) (string, bool) {
	// Defensive check: ensure config is not nil
	if config == nil {
		klog.V(5).Infof("Cannot retrieve service account token: config is nil")
		return "", false
	}

	klog.V(5).Infof("Attempting to retrieve service account token for inventory service authentication")

	// Create a Kubernetes clientset using the existing config (with client certs)
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.V(5).Infof("Failed to create Kubernetes clientset: %v", err)
		return "", false
	}

	namespace := "konveyor-forklift"
	serviceAccountName := "forklift-controller"

	// Try method 1: Get a secret named exactly "forklift-controller-token"
	// (older Kubernetes versions create this automatically)
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, serviceAccountName+"-token", metav1.GetOptions{})
	if err == nil && secret.Type == corev1.SecretTypeServiceAccountToken {
		if token, ok := secret.Data["token"]; ok && len(token) > 0 {
			klog.V(5).Infof("Found service account token in secret %s/%s (length: %d)",
				namespace, secret.Name, len(token))
			return string(token), true
		}
	}

	// Try method 2: List all secrets and find one associated with the service account
	// This handles:
	// - Auto-generated secrets with random suffixes (forklift-controller-token-xxxxx)
	// - Manually created secrets with service account annotations
	klog.V(5).Infof("Secret %s not found, listing all secrets in namespace %s", serviceAccountName+"-token", namespace)

	secrets, err := clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.V(5).Infof("Failed to list secrets in namespace %s: %v", namespace, err)
		return "", false
	}

	for _, s := range secrets.Items {
		// Check if this secret is associated with our service account and is the correct type
		// Match by name prefix or by annotation
		isServiceAccountToken := false

		if strings.HasPrefix(s.Name, serviceAccountName+"-token-") {
			isServiceAccountToken = true
		} else if s.Annotations != nil &&
			s.Annotations[corev1.ServiceAccountNameKey] == serviceAccountName {
			isServiceAccountToken = true
		}

		// Only process secrets of the correct type
		if isServiceAccountToken && s.Type == corev1.SecretTypeServiceAccountToken {
			if token, ok := s.Data["token"]; ok && len(token) > 0 {
				klog.V(5).Infof("Found service account token in secret %s/%s (length: %d)",
					namespace, s.Name, len(token))
				return string(token), true
			}
		}
	}

	klog.V(5).Infof("No service account token found for %s/%s", namespace, serviceAccountName)
	return "", false
}

// NeedsBearerTokenForInventory determines if we need to retrieve a bearer token
// for inventory service authentication.
//
// Returns true when:
// - Using client certificate authentication (CertData or CertFile is set)
// - AND no bearer token is already configured
//
// This indicates a Kind/minikube-style cluster where the inventory service
// needs a bearer token instead of client certificates.
func NeedsBearerTokenForInventory(config *rest.Config) bool {
	// Defensive check: ensure config is not nil
	if config == nil {
		return false
	}

	usingClientCert := (config.CertData != nil || config.CertFile != "")
	hasBearerToken := (config.BearerToken != "" || config.BearerTokenFile != "")

	return usingClientCert && !hasBearerToken
}
