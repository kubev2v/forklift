package validator

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

type Validator struct {
	*plancontext.Context
	log logging.LevelLogger
}

func New(ctx *plancontext.Context) *Validator {
	log := logging.WithName("validator|azure")
	return &Validator{
		Context: ctx,
		log:     log,
	}
}

func (r *Validator) Validate(vmRef ref.Ref) (ok bool, err error) {
	if ok, err = r.validateStorage(vmRef); !ok || err != nil {
		return
	}

	if ok, err = r.NetworksMapped(vmRef); !ok || err != nil {
		return
	}

	if ok, err = r.StorageMapped(vmRef); !ok || err != nil {
		return
	}

	return true, nil
}
