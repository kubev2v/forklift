package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

// ProcessTemplate reads a file and processes it as a Go template using the provided variables.
func ProcessTemplate(filePath string, vars map[string]string, leftDelim, rightDelim string) ([]byte, error) {
	rawData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	tmpl, err := template.New("template").Delims(leftDelim, rightDelim).Parse(string(rawData))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.Bytes(), nil
}

// DecodeDeployment decodes YAML data into an appsv1.Deployment object.
func DecodeDeployment(data []byte) (*appsv1.Deployment, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 1024)
	var deploy appsv1.Deployment
	if err := decoder.Decode(&deploy); err != nil {
		return nil, fmt.Errorf("failed to decode deployment: %w", err)
	}
	return &deploy, nil
}

// EnsureNamespace makes sure a namespace exists; if not, it creates it.
func EnsureNamespace(clientset *kubernetes.Clientset, namespace string) error {
	_, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
	if err != nil {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
		}
		fmt.Println("Created namespace:", namespace)
	}
	return nil
}

// EnsureServiceAccount makes sure a ServiceAccount exists in the given namespace.
func EnsureServiceAccount(clientset *kubernetes.Clientset, namespace, saName string) error {
	_, err := clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), saName, metav1.GetOptions{})
	if err != nil {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      saName,
				Namespace: namespace,
			},
		}
		_, err = clientset.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), sa, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create service account %s in namespace %s: %w", saName, namespace, err)
		}
		fmt.Println("Created service account:", saName)
	}
	return nil
}

// EnsureClusterRole creates a ClusterRole if it does not exist.
func EnsureClusterRole(clientset *kubernetes.Clientset, role *rbacv1.ClusterRole) error {
	_, err := clientset.RbacV1().ClusterRoles().Get(context.TODO(), role.Name, metav1.GetOptions{})
	if err != nil {
		created, err := clientset.RbacV1().ClusterRoles().Create(context.TODO(), role, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster role %s: %w", role.Name, err)
		}
		fmt.Printf("ClusterRole %q created.\n", created.Name)
	} else {
		fmt.Printf("ClusterRole %q already exists.\n", role.Name)
	}
	return nil
}

// EnsureClusterRoleBinding creates a ClusterRoleBinding if it does not exist.
func EnsureClusterRoleBinding(clientset *kubernetes.Clientset, binding *rbacv1.ClusterRoleBinding) error {
	_, err := clientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), binding.Name, metav1.GetOptions{})
	if err != nil {
		created, err := clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), binding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster role binding %s: %w", binding.Name, err)
		}
		fmt.Printf("ClusterRoleBinding %q created.\n", created.Name)
	} else {
		fmt.Printf("ClusterRoleBinding %q already exists.\n", binding.Name)
	}
	return nil
}

// EnsureDeployment ensures that an appsv1.Deployment exists in the specified namespace.
// If the deployment doesn't exist, it creates it; otherwise, it prints that the deployment exists.
func EnsureDeployment(clientset *kubernetes.Clientset, namespace string, deploy *appsv1.Deployment) error {
	// Attempt to get the deployment by name.
	existing, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploy.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Deployment does not exist, so create it.
			created, err := clientset.AppsV1().Deployments(namespace).Create(context.TODO(), deploy, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create deployment %s: %w", deploy.Name, err)
			}
			fmt.Println("Deployment created:", created.Name)
			return nil
		}
		// Other errors when retrieving the deployment.
		return fmt.Errorf("failed to get deployment %s: %w", deploy.Name, err)
	}

	// Deployment already exists.
	fmt.Println("Deployment already exists:", existing.Name)
	return nil
}

func EnsureRoleBinding(clientset *kubernetes.Clientset, binding *rbacv1.RoleBinding) error {
	// Retrieve the RoleBinding in the specified namespace.
	_, err := clientset.RbacV1().RoleBindings(binding.Namespace).Get(context.TODO(), binding.Name, metav1.GetOptions{})
	if err != nil {
		// Create the RoleBinding if it does not exist.
		created, err := clientset.RbacV1().RoleBindings(binding.Namespace).Create(context.TODO(), binding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create role binding %s: %w", binding.Name, err)
		}
		fmt.Printf("RoleBinding %q created.\n", created.Name)
	} else {
		fmt.Printf("RoleBinding %q already exists.\n", binding.Name)
	}
	return nil
}

func EnsureSecret(clientset *kubernetes.Clientset, secret *corev1.Secret) error {
	// Attempt to get the Secret.
	_, err := clientset.CoreV1().Secrets(secret.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
	if err != nil {
		// Create the Secret if it does not exist.
		created, err := clientset.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create secret %s: %w", secret.Name, err)
		}
		fmt.Printf("Secret %q created.\n", created.Name)
	} else {
		fmt.Printf("Secret %q already exists.\n", secret.Name)
	}
	return nil
}
