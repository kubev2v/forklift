package plan

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensure the namespace exists on the destination.
func ensureNamespace(plan *api.Plan, client client.Client) error {
	ns := &core.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: plan.Spec.TargetNamespace,
		},
	}
	err := client.Create(context.TODO(), ns)
	if err != nil && k8serr.IsAlreadyExists(err) {
		err = nil
	}
	return err
}

// Ensure the config map exists on the destination
func ensureConfigMap(cm *core.ConfigMap, name func(plan *api.Plan) string, plan *api.Plan, client client.Client) error {
	cm.ObjectMeta = meta.ObjectMeta{
		Name:      name(plan),
		Namespace: plan.Spec.TargetNamespace,
	}
	err := client.Create(context.TODO(), cm)
	if err != nil && k8serr.IsAlreadyExists(err) {
		err = nil
	}
	return err
}
