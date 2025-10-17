package vsphere

import (
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/ensurer"
)

// vSphere adapter.
type Adapter struct{}

// Constructs a vSphere builder.
func (r *Adapter) Builder(ctx *plancontext.Context) (builder base.Builder, err error) {
	b := &Builder{Context: ctx}
	err = b.Load()
	if err != nil {
		return
	}
	builder = b
	return
}

// Constructs a ensurer.
func (r *Adapter) Ensurer(ctx *plancontext.Context) (ensure base.Ensurer, err error) {
	e := &ensurer.Ensurer{Context: ctx}
	ensure = e
	return
}

// Constructs a vSphere validator.
func (r *Adapter) Validator(ctx *plancontext.Context) (validator base.Validator, err error) {
	v := &Validator{Context: ctx}
	validator = v
	return
}

// Constructs a vSphere client.
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
