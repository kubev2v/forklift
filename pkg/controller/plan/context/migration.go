package context

import (
	"context"
	"path"

	"github.com/go-logr/logr"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	core "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Not enough data to build the context.
type NotEnoughDataError struct {
}

func (e NotEnoughDataError) Error() string {
	return "Not enough data to build plan context."
}

// Factory.
func New(
	client k8sclient.Client, plan *api.Plan, log logr.Logger) (ctx *Context, err error) {
	ctx = &Context{
		Client:    client,
		Plan:      plan,
		Migration: &api.Migration{},
		Log:       log,
	}
	err = ctx.build()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

// Plan execution context.
type Context struct {
	// Host client.
	k8sclient.Client
	// Plan.
	Plan *api.Plan
	// Map.
	Map struct {
		// Network
		Network *api.NetworkMap
		// Storage
		Storage *api.StorageMap
	}
	// Migration
	Migration *api.Migration
	// Source.
	Source Source
	// Destination.
	Destination Destination
	// Hooks.
	Hooks []*api.Hook
	// Logger.
	Log logr.Logger
}

// Build.
func (r *Context) build() (err error) {
	r.Map.Network = r.Plan.Referenced.Map.Network
	if r.Map.Network == nil {
		err = liberr.Wrap(NotEnoughDataError{})
		return
	}
	r.Map.Storage = r.Plan.Referenced.Map.Storage
	if r.Map.Storage == nil {
		err = liberr.Wrap(NotEnoughDataError{})
		return
	}
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
	r.Hooks = r.Plan.Hooks

	return
}

// Set the migration.
// This will update the logger context.
func (r *Context) SetMigration(migration *api.Migration) {
	if migration == nil {
		return
	}
	r.Migration = migration
	r.Log = r.Log.WithValues(
		"migration",
		path.Join(
			migration.Namespace,
			migration.Name))
}

// Source.
type Source struct {
	// Provider
	Provider *api.Provider
	// Provider API client.
	Inventory web.Client
	// Provider Secret.
	Secret *core.Secret
}

// Build.
// Returns: NotEnoughDataError when:
//
//	Plan.Referenced.Source is not complete.
func (r *Source) build(ctx *Context) (err error) {
	r.Provider = ctx.Plan.Referenced.Provider.Source
	if r.Provider == nil {
		err = liberr.Wrap(NotEnoughDataError{})
		return
	}

	if !r.Provider.IsHost() {
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

// Build.
// Returns: NotEnoughDataError when:
//
//	Plan.Referenced.Destination is not complete.
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
		r.Client, err = r.Provider.Client(nil)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}
	r.Inventory, err = web.NewClient(r.Provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

// Find a Hook by ref.
func (r *Context) FindHook(ref core.ObjectReference) (hook *api.Hook, found bool) {
	for _, h := range r.Hooks {
		if h.Namespace == ref.Namespace && h.Name == ref.Name {
			found = true
			hook = h
			break
		}
	}

	return
}
