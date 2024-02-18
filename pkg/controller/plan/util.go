package plan

import (
	"context"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
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
