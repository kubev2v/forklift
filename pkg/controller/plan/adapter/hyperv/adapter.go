// Package hyperv provides the HyperV plan adapter.
// HyperV uses WinRM/PowerShell for inventory and -i disk mode for virt-v2v.
package hyperv

import (
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/ensurer"
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
	c := &Client{Context: ctx}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

func (r *Adapter) DestinationClient(ctx *plancontext.Context) (base.DestinationClient, error) {
	return &DestinationClient{Context: ctx}, nil
}
