package ovirt

import (
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/ensurer"
)

// oVirt adapter.
type Adapter struct{}

// Constructs a oVirt builder.
func (r *Adapter) Builder(ctx *plancontext.Context) (builder base.Builder, err error) {
	builder = &Builder{Context: ctx}
	return
}

// Constructs a oVirt validator.
func (r *Adapter) Validator(ctx *plancontext.Context) (validator base.Validator, err error) {
	v := &Validator{Context: ctx}
	validator = v
	return
}

// Constructs an oVirt client.
func (r *Adapter) Client(ctx *plancontext.Context) (client base.Client, err error) {
	c := &Client{Context: ctx}
	err = c.connect()
	if err != nil {
		return
	}
	client = c
	return
}

// Constructs a ensurer.
func (r *Adapter) Ensurer(ctx *plancontext.Context) (ensure base.Ensurer, err error) {
	e := &ensurer.Ensurer{Context: ctx}
	ensure = e
	return
}

// Constucts a destination client.
func (r *Adapter) DestinationClient(ctx *plancontext.Context) (destinationClient base.DestinationClient, err error) {
	destinationClient = &DestinationClient{Context: ctx}
	return
}
