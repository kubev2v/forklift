package ensurer

import (
	"context"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Ensurer struct {
	*plancontext.Context
	log logging.LevelLogger
}

func New(ctx *plancontext.Context) *Ensurer {
	log := logging.WithName("ensurer|azure")
	return &Ensurer{
		Context: ctx,
		log:     log,
	}
}

// ensureCreated is a helper that creates a resource if it doesn't already exist.
// Returns true if the resource already existed, false if it was created.
func (r *Ensurer) ensureCreated(obj client.Object, logName string) (existed bool, err error) {
	ctx := context.TODO()
	existing := obj.DeepCopyObject().(client.Object)
	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	err = r.Destination.Client.Get(ctx, key, existing)
	if err == nil {
		r.log.Info("Resource already exists", "kind", logName, "name", obj.GetName())
		return true, nil
	}
	if !k8serr.IsNotFound(err) {
		return false, liberr.Wrap(err)
	}

	err = r.Destination.Client.Create(ctx, obj)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	return false, nil
}

func (r *Ensurer) SharedConfigMaps(vm *planapi.VMStatus, configMaps []core.ConfigMap) error {
	return nil
}

func (r *Ensurer) SharedSecrets(vm *planapi.VMStatus, secrets []core.Secret) error {
	return nil
}

func (r *Ensurer) PersistentVolumeClaims(vm *planapi.VMStatus, pvcs []core.PersistentVolumeClaim) error {
	return nil
}
