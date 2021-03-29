package network

import (
	libcnd "github.com/konveyor/controller/pkg/condition"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/map/network/handler"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type MapPredicate struct {
	predicate.Funcs
}

func (r MapPredicate) Create(e event.CreateEvent) bool {
	_, cast := e.Object.(*api.NetworkMap)
	if cast {
		libref.Mapper.Create(e)
		return true
	}

	return false
}

func (r MapPredicate) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*api.NetworkMap)
	if !cast {
		return false
	}
	changed := object.Status.ObservedGeneration < object.Generation
	if changed {
		libref.Mapper.Update(e)
	}

	return changed
}

func (r MapPredicate) Delete(e event.DeleteEvent) bool {
	_, cast := e.Object.(*api.NetworkMap)
	if cast {
		libref.Mapper.Delete(e)
		return true
	}

	return false
}

//
// Provider watch predicate.
// Also ensures an inventory watch is created and
// associated with the channel source.
type ProviderPredicate struct {
	handler.WatchManager
	predicate.Funcs
	channel chan event.GenericEvent
	client  client.Client
}

//
// Provider created event.
func (r *ProviderPredicate) Create(e event.CreateEvent) bool {
	p, cast := e.Object.(*api.Provider)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

//
// Provider updated event.
func (r *ProviderPredicate) Update(e event.UpdateEvent) bool {
	p, cast := e.ObjectNew.(*api.Provider)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		if reconciled {
			r.ensureWatch(p)
		}
		return reconciled
	}

	return false
}

//
// Provider deleted event.
func (r *ProviderPredicate) Delete(e event.DeleteEvent) bool {
	p, cast := e.Object.(*api.Provider)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

//
// Generic provider watch event.
func (r *ProviderPredicate) Generic(e event.GenericEvent) bool {
	p, cast := e.Object.(*api.Provider)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

//
// Ensure the there is a watch for the provider and kind.
func (r *ProviderPredicate) ensureWatch(p *api.Provider) {
	if !p.Status.HasCondition(libcnd.Ready) {
		return
	}
	h, err := handler.New(r.client, r.channel, p)
	if err != nil {
		log.Trace(err)
		return
	}
	err = h.Watch(&r.WatchManager)
	if err != nil {
		log.Trace(err)
		return
	}
}
