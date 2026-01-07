package validator

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

// Validator validates EC2 VM migration prerequisites before migration starts.
// Checks storage (EBS volumes exist, instance store detected), network/storage mapping completeness.
type Validator struct {
	*plancontext.Context                     // Plan context with provider inventory and mappings
	log                  logging.LevelLogger // Structured logger for validation issues
}

// New creates a new EC2 Validator with plan context for inventory access and mapping validation.
func New(ctx *plancontext.Context) *Validator {
	log := logging.WithName("validator|ec2")
	return &Validator{
		Context: ctx,
		log:     log,
	}
}

// Validate performs all prerequisite checks for VM migration: storage, network mappings, storage mappings.
// Stops at first error. All checks must pass for migration to proceed.
func (r *Validator) Validate(vmRef ref.Ref) (ok bool, err error) {
	// Validate storage: EBS volumes exist and are usable
	if ok, err = r.validateStorage(vmRef); !ok || err != nil {
		return
	}

	// Validate network mappings are complete
	if ok, err = r.NetworksMapped(vmRef); !ok || err != nil {
		return
	}

	// Validate storage mappings are complete
	if ok, err = r.StorageMapped(vmRef); !ok || err != nil {
		return
	}

	return true, nil
}
