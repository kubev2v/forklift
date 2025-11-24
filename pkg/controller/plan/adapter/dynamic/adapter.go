package dynamic

import (
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/ensurer"
)

// Adapter is the generic dynamic provider adapter.
// It works with ANY dynamic provider using schema from the Provider CRD.
type Adapter struct{}

// Builder constructs a generic builder using the schema.
func (r *Adapter) Builder(ctx *plancontext.Context) (builder base.Builder, err error) {
	// Load schema from provider
	schema, err := LoadSchema(ctx.Source.Provider)
	if err != nil {
		return
	}

	// Validate schema meets minimum contract
	if err = schema.ValidateMinimumContract(); err != nil {
		return
	}

	b := NewBuilder(ctx, schema)
	builder = b
	return
}

// Validator constructs a generic validator using the schema.
func (r *Adapter) Validator(ctx *plancontext.Context) (validator base.Validator, err error) {
	// Load schema from provider
	schema, err := LoadSchema(ctx.Source.Provider)
	if err != nil {
		return
	}

	// Validate schema meets minimum contract
	if err = schema.ValidateMinimumContract(); err != nil {
		return
	}

	v := NewValidator(ctx, schema)
	validator = v
	return
}

// Client constructs a client for the dynamic provider.
func (r *Adapter) Client(ctx *plancontext.Context) (client base.Client, err error) {
	c := &Client{Context: ctx}
	client = c
	return
}

// DestinationClient constructs a destination client.
func (r *Adapter) DestinationClient(ctx *plancontext.Context) (destinationClient base.DestinationClient, err error) {
	dc := &DestinationClient{Context: ctx}
	destinationClient = dc
	return
}

// Ensurer constructs an ensurer.
func (r *Adapter) Ensurer(ctx *plancontext.Context) (ensure base.Ensurer, err error) {
	e := &ensurer.Ensurer{Context: ctx}
	ensure = e
	return
}
