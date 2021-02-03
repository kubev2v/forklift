package context

import (
	"context"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	core "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Not enough data to build the context.
type NotEnoughDataError struct {
}

func (e NotEnoughDataError) Error() string {
	return "Not enough data to build plan context."
}

//
// Factory.
func New(client k8sclient.Client, plan *api.Plan, migration *api.Migration) (ctx *Context, err error) {
	ctx = &Context{
		Client:    client,
		Plan:      plan,
		Migration: migration,
	}
	err = ctx.build()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

//
// Plan execution context.
type Context struct {
	// Host client.
	k8sclient.Client
	// Plan.
	Plan *api.Plan
	// Migration
	Migration *api.Migration
	// Source.
	Source Source
	// Destination.
	Destination Destination
}

//
// Build.
func (r *Context) build() (err error) {
	err = r.Source.build(r)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.Destination.build(r)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

//
// Source.
type Source struct {
	// Provider
	Provider *api.Provider
	// Provider API client.
	Inventory web.Client
	// Provider Secret.
	Secret *core.Secret
}

//
// Build.
// Returns: NotEnoughDataError when:
//   Plan.Referenced.Source is not complete.
func (r *Source) build(ctx *Context) (err error) {
	r.Provider = ctx.Plan.Referenced.Provider.Source
	if r.Provider == nil {
		err = liberr.Wrap(NotEnoughDataError{})
		return
	}
	ref := r.Provider.Spec.Secret
	r.Secret = &core.Secret{}
	err = ctx.Get(
		context.TODO(),
		k8sclient.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		r.Secret)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.Inventory, err = web.NewClient(r.Provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

// Destination.
type Destination struct {
	// Remote client.
	k8sclient.Client
	// Provider.
	Provider *api.Provider
	// Provider API client.
	Inventory web.Client
}

//
// Build.
// Returns: NotEnoughDataError when:
//   Plan.Referenced.Destination is not complete.
func (r *Destination) build(ctx *Context) (err error) {
	r.Provider = ctx.Plan.Referenced.Provider.Destination
	if r.Provider == nil {
		err = liberr.Wrap(NotEnoughDataError{})
		return
	}
	if !r.Provider.IsHost() {
		ref := r.Provider.Spec.Secret
		secret := &core.Secret{}
		err = ctx.Get(
			context.TODO(),
			k8sclient.ObjectKey{
				Namespace: ref.Namespace,
				Name:      ref.Name,
			},
			secret)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Client, err = r.Provider.Client(secret)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	} else {
		r.Client = ctx
	}
	r.Inventory, err = web.NewClient(r.Provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}
