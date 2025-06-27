package k8s

import (
	"certificate-tool/internal/utils"
	"context"
	"fmt"

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

// EnsurePopulatorPod creates or reapplies a populator Pod mounting its PVC.
func EnsurePopulatorPod(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, image, testLabel string, vm utils.VM, storageVendorProduct, pvcName string) error {
	pods := clientset.CoreV1().Pods(namespace)
	_, err := pods.Get(ctx, podName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		mustBeDefined := false
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
				Labels:    map[string]string{"test": testLabel},
			},
			Spec: corev1.PodSpec{
				RestartPolicy:      corev1.RestartPolicyNever,
				ServiceAccountName: "populator",
				Volumes: []corev1.Volume{
					{Name: "target", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}},
				},
				Containers: []corev1.Container{{
					Name:            "populate",
					Image:           image,
					ImagePullPolicy: corev1.PullAlways,
					VolumeDevices:   []corev1.VolumeDevice{{Name: "target", DevicePath: "/dev/block"}},
					Ports:           []corev1.ContainerPort{{Name: "metrics", ContainerPort: 8443, Protocol: corev1.ProtocolTCP}},
					EnvFrom:         []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{corev1.LocalObjectReference{Name: "populator-secret"}, &mustBeDefined}}},
					Args: []string{
						// name or id is fine, the govmomi code uses a finder
						fmt.Sprintf("--source-vm-id=%s", vm.Name),
						fmt.Sprintf("--source-vmdk=%s", vm.VmdkPath),
						fmt.Sprintf("--target-namespace=%s", namespace),
						fmt.Sprintf("--cr-name=%s", "notrequired"),
						fmt.Sprintf("--cr-namespace=%s", "notrequired"),
						fmt.Sprintf("--owner-name=%s", pvcName),
						fmt.Sprintf("--secret-name=%s-secret", "notrequired"),
						fmt.Sprintf("--pvc-size=%s", vm.Size),
						fmt.Sprintf("--owner-uid=%s", "notrequired"),
						fmt.Sprintf("--storage-vendor-product=%s", storageVendorProduct),
					},
				}},
			},
		}
		if _, err := pods.Create(ctx, pod, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create populator pod %s: %w", podName, err)
		}
		klog.Infof("Created populator pod %s", podName)
		return nil
	}
	if err != nil {
		return fmt.Errorf("error getting pod %s: %w", podName, err)
	}
	klog.Infof("Populator pod %s already exists", podName)
	return nil
}
