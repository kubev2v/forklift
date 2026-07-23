package ensurer

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	core "k8s.io/api/core/v1"
)

var _ base.Ensurer = &Ensurer{}

type Ensurer struct {
	*plancontext.Context
}

func New(ctx *plancontext.Context) *Ensurer {
	return &Ensurer{Context: ctx}
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
