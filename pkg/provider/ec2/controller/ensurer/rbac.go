package ensurer

import (
	"context"
	"fmt"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsurePopulatorServiceAccount creates RBAC resources for EC2 volume populator pods.
// Creates ServiceAccount, Role (PVC read), RoleBinding, ClusterRole (PV/CR access), and ClusterRoleBinding.
// Populator needs permissions to read PVCs, create PVs after snapshot-to-volume conversion, and update CR status.
// Idempotent - skips creation if resources exist. Safe for multiple calls.
func (r *Ensurer) EnsurePopulatorServiceAccount(ctx context.Context, namespace string) error {
	r.log.Info("Ensuring ServiceAccount for EC2 populator", "namespace", namespace)

	sa := core.ServiceAccount{
		ObjectMeta: meta.ObjectMeta{
			Name:      "populator",
			Namespace: namespace,
		},
	}
	err := r.Client.Create(ctx, &sa)
	if err != nil && !k8serr.IsAlreadyExists(err) {
		return liberr.Wrap(err, "failed to create populator ServiceAccount")
	}

	role := rbacv1.Role{
		ObjectMeta: meta.ObjectMeta{
			Name:      "populator-pvc-reader",
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
	err = r.Client.Create(ctx, &role)
	if err != nil && !k8serr.IsAlreadyExists(err) {
		return liberr.Wrap(err, "failed to create populator Role")
	}

	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: meta.ObjectMeta{
			Name:      "populator-pvc-reader-binding",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "populator",
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     "populator-pvc-reader",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	err = r.Client.Create(ctx, &roleBinding)
	if err != nil && !k8serr.IsAlreadyExists(err) {
		return liberr.Wrap(err, "failed to create populator RoleBinding")
	}

	// EC2 populator creates PVs directly after creating EBS volumes
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: meta.ObjectMeta{
			Name: "ec2-populator-pv-creator",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"get", "create", "update", "patch"},
			},
			{
				APIGroups: []string{"forklift.konveyor.io"},
				Resources: []string{"ec2volumepopulators"},
				Verbs:     []string{"get", "list", "watch", "update", "patch"},
			},
		},
	}
	err = r.Client.Create(ctx, &clusterRole)
	if err != nil && !k8serr.IsAlreadyExists(err) {
		return liberr.Wrap(err, "failed to create populator ClusterRole")
	}

	clusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: meta.ObjectMeta{
			Name: fmt.Sprintf("ec2-populator-pv-creator-%s", namespace),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "populator",
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "ec2-populator-pv-creator",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	err = r.Client.Create(ctx, &clusterRoleBinding)
	if err != nil && !k8serr.IsAlreadyExists(err) {
		return liberr.Wrap(err, "failed to create populator ClusterRoleBinding")
	}

	r.log.Info("Populator ServiceAccount and RBAC configured", "namespace", namespace)
	return nil
}
