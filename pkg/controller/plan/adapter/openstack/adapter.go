package openstack

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
)

// Openstack adapter.
type Adapter struct{}

// Constructs a openstack builder.
func (r *Adapter) Builder(ctx *plancontext.Context) (builder base.Builder, err error) {
	builder = &Builder{Context: ctx}
	return
}

// Constructs a openstack validator.
func (r *Adapter) Validator(plan *api.Plan) (validator base.Validator, err error) {
	v := &Validator{plan: plan}
	err = v.Load()
	if err != nil {
		return
	}
	validator = v
	return
}

// Constructs an openstack client.
func (r *Adapter) Client(ctx *plancontext.Context) (client base.Client, err error) {
	c := &Client{
		Context: ctx,
	}
	c.Log = ctx.Log.WithName("client")
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
