package ovfbase

import (
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/ensurer"
	core "k8s.io/api/core/v1"
)

// Adapter for OVF-based providers.
type Adapter struct{}

// Constructs a builder for OVF-based migrations.
func (r *Adapter) Builder(ctx *plancontext.Context) (builder base.Builder, err error) {
	b := &Builder{Context: ctx}
	builder = b
	return
}

// Constructs an ensurer.
func (r *Adapter) Ensurer(ctx *plancontext.Context) (ensure base.Ensurer, err error) {
	e := &ensurer.Ensurer{Context: ctx}
	ensure = e
	return
}

// Constructs a validator for OVF-based migrations.
func (r *Adapter) Validator(ctx *plancontext.Context) (validator base.Validator, err error) {
	v := &Validator{Context: ctx}
	validator = v
	return
}

// Constructs a client for OVF-based provider communication.
func (r *Adapter) Client(ctx *plancontext.Context) (client base.Client, err error) {
	c := &Client{Context: ctx}
	err = c.connect()
	if err != nil {
		return
	}
	client = c
	return
}

// Constucts a destination client.
func (r *Adapter) DestinationClient(ctx *plancontext.Context) (destinationClient base.DestinationClient, err error) {
	destinationClient = &DestinationClient{Context: ctx}
	return
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
