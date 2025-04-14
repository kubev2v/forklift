package k8s

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewRole returns a Role that allows managing PVCs in the given namespace.
func NewRole(roleName, namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch", "patch", "create", "update", "delete"},
			},
		},
	}
}

// NewClusterRole returns a ClusterRole for managing various K8s and Forklift resources.
func NewClusterRole(roleName string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "persistentvolumeclaims", "persistentvolumes", "storageclasses", "secrets"},
				Verbs:     []string{"get", "list", "watch", "patch", "create", "update", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch", "update"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"forklift.konveyor.io"},
				Resources: []string{"ovirtvolumepopulators", "vspherexcopyvolumepopulators", "openstackvolumepopulators"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}
}

// NewClusterRoleBinding returns a ClusterRoleBinding that binds the SA to the given ClusterRole.
func NewClusterRoleBinding(namespace, roleName, saName string) *rbacv1.ClusterRoleBinding {
	bindingName := fmt.Sprintf("%s-binding", roleName)
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
	}
}

// NewRoleBinding returns a RoleBinding that binds the SA to the Role in the namespace.
func NewRoleBinding(namespace, saName, roleName string) *rbacv1.RoleBinding {
	bindingName := fmt.Sprintf("%s-binding", roleName)
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
	}
}
