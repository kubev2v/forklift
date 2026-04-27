// Package hyperv provides the HyperV plan adapter.
// HyperV uses WinRM/PowerShell for inventory and -i disk mode for virt-v2v.
package hyperv

import (
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/ensurer"
	core "k8s.io/api/core/v1"
)

type Adapter struct{}

func (r *Adapter) Builder(ctx *plancontext.Context) (base.Builder, error) {
	return &Builder{Context: ctx}, nil
}

func (r *Adapter) Ensurer(ctx *plancontext.Context) (base.Ensurer, error) {
	return &ensurer.Ensurer{Context: ctx}, nil
}

func (r *Adapter) Validator(ctx *plancontext.Context) (base.Validator, error) {
	return &Validator{Context: ctx}, nil
}

func (r *Adapter) Client(ctx *plancontext.Context) (base.Client, error) {
	return &Client{Context: ctx}, nil
}

func (r *Adapter) DestinationClient(ctx *plancontext.Context) (base.DestinationClient, error) {
	return &DestinationClient{Context: ctx}, nil
}

// Constructs a storage mapper (no-op).
func (r *Adapter) StorageMapper(ctx *plancontext.Context) (base.StorageMapper, error) {
	return &NoOpStorageMapper{}, nil
}

// NoOpStorageMapper is a no-op implementation for providers that don't use copy-offload.
type NoOpStorageMapper struct{}

func (r *NoOpStorageMapper) IsCopyOffload(diskFile string, vmID string) bool {
	return false
}

func (r *NoOpStorageMapper) IsPVCCopyOffload(pvc *core.PersistentVolumeClaim) bool {
	return false
}

func (r *NoOpStorageMapper) IsAnyPVCCopyOffload(pvcs []*core.PersistentVolumeClaim) bool {
	return false
}
