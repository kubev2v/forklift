package ova

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
)

// OVA adapter.
type Adapter struct{}

// Constructs a OVA builder.
func (r *Adapter) Builder(ctx *plancontext.Context) (builder base.Builder, err error) {
	b := &Builder{Context: ctx}
	builder = b
	return
}

// Constructs a OVA validator.
func (r *Adapter) Validator(plan *api.Plan) (validator base.Validator, err error) {
	v := &Validator{plan: plan}
	err = v.Load()
	if err != nil {
		return
	}
	validator = v
	return
}

// Constructs a OVA client.
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
