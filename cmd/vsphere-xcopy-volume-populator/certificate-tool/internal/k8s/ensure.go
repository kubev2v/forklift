package k8s

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// EnsureNamespace makes sure a namespace exists; if not, it creates it.
func EnsureNamespace(clientset *kubernetes.Clientset, namespace string) error {
	_, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create namespace %q: %w", namespace, err)
		}
		klog.Infof("Namespace %q created", namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get namespace %q: %w", namespace, err)
	}
	klog.Infof("Namespace %q already exists", namespace)
	return nil
}

// EnsureServiceAccount ensures a ServiceAccount exists in the given namespace.
func EnsureServiceAccount(clientset *kubernetes.Clientset, namespace, saName string) error {
	_, err := clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), saName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: saName, Namespace: namespace},
		}
		_, err = clientset.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), sa, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ServiceAccount %q in namespace %q: %w", saName, namespace, err)
		}
		klog.Infof("ServiceAccount %q created in namespace %q", saName, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get ServiceAccount %q in namespace %q: %w", saName, namespace, err)
	}
	klog.Infof("ServiceAccount %q already exists in namespace %q", saName, namespace)
	return nil
}

func EnsureClusterRole(clientset *kubernetes.Clientset, role *rbacv1.ClusterRole) error {
	_, err := clientset.RbacV1().ClusterRoles().Get(context.TODO(), role.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		created, err := clientset.RbacV1().ClusterRoles().Create(context.TODO(), role, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ClusterRole %q: %w", role.Name, err)
		}
		klog.Infof("ClusterRole %q created", created.Name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get ClusterRole %q: %w", role.Name, err)
	}
	klog.Infof("ClusterRole %q already exists", role.Name)
	return nil
}

func EnsureClusterRoleBinding(clientset *kubernetes.Clientset, binding *rbacv1.ClusterRoleBinding) error {
	_, err := clientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), binding.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		created, err := clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), binding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ClusterRoleBinding %q: %w", binding.Name, err)
		}
		klog.Infof("ClusterRoleBinding %q created", created.Name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get ClusterRoleBinding %q: %w", binding.Name, err)
	}
	klog.Infof("ClusterRoleBinding %q already exists", binding.Name)
	return nil
}

func EnsureRole(clientset *kubernetes.Clientset, role *rbacv1.Role) error {
	_, err := clientset.RbacV1().Roles(role.Namespace).Get(context.TODO(), role.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		created, err := clientset.RbacV1().Roles(role.Namespace).Create(context.TODO(), role, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Role %q in namespace %q: %w", role.Name, role.Namespace, err)
		}
		klog.Infof("Role %q created in namespace %q", created.Name, created.Namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get Role %q in namespace %q: %w", role.Name, role.Namespace, err)
	}
	klog.Infof("Role %q already exists in namespace %q", role.Name, role.Namespace)
	return nil
}

func EnsureDeployment(clientset *kubernetes.Clientset, namespace string, deploy *appsv1.Deployment) error {
	existing, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploy.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		created, err := clientset.AppsV1().Deployments(namespace).Create(context.TODO(), deploy, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Deployment %q: %w", deploy.Name, err)
		}
		klog.Infof("Deployment %q created", created.Name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get Deployment %q: %w", deploy.Name, err)
	}
	klog.Infof("Deployment %q already exists", existing.Name)
	return nil
}

func EnsureRoleBinding(clientset *kubernetes.Clientset, binding *rbacv1.RoleBinding) error {
	_, err := clientset.RbacV1().RoleBindings(binding.Namespace).Get(context.TODO(), binding.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		created, err := clientset.RbacV1().RoleBindings(binding.Namespace).Create(context.TODO(), binding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create RoleBinding %q: %w", binding.Name, err)
		}
		klog.Infof("RoleBinding %q created", created.Name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get RoleBinding %q: %w", binding.Name, err)
	}
	klog.Infof("RoleBinding %q already exists", binding.Name)
	return nil
}

func EnsureSecret(clientset *kubernetes.Clientset, secret *corev1.Secret) error {
	_, err := clientset.CoreV1().Secrets(secret.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		created, err := clientset.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Secret %q: %w", secret.Name, err)
		}
		klog.Infof("Secret %q created", created.Name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get Secret %q: %w", secret.Name, err)
	}
	klog.Infof("Secret %q already exists", secret.Name)
	return nil
}

func EnsurePersistentVolumeClaim(clientset *kubernetes.Clientset, namespace string, pvc *corev1.PersistentVolumeClaim) error {
	existing, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvc.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		created, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create PVC %q: %w", pvc.Name, err)
		}
		klog.Infof("PVC %q created", created.Name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get PVC %q: %w", pvc.Name, err)
	}
	klog.Infof("PVC %q already exists", existing.Name)
	return nil
}
