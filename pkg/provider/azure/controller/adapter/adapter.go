package adapter

import (
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/builder"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/client"
	azureensurer "github.com/kubev2v/forklift/pkg/provider/azure/controller/ensurer"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/validator"
)

type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (r *Adapter) Ensurer(ctx *plancontext.Context) (base.Ensurer, error) {
	return azureensurer.New(ctx), nil
}

func (r *Adapter) Builder(ctx *plancontext.Context) (base.Builder, error) {
	return builder.New(ctx), nil
}

func (r *Adapter) Validator(ctx *plancontext.Context) (base.Validator, error) {
	return validator.New(ctx), nil
}

func (r *Adapter) Client(ctx *plancontext.Context) (base.Client, error) {
	c := &client.Client{Context: ctx}
	if err := c.Connect(); err != nil {
		return nil, err
	}
	return c, nil
}

func (r *Adapter) DestinationClient(ctx *plancontext.Context) (base.DestinationClient, error) {
	return &DestinationClient{Context: ctx}, nil
}
