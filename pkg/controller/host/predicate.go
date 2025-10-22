package host

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/host/handler"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type HostPredicate struct {
	predicate.TypedFuncs[*api.Host]
}

func (r HostPredicate) Create(e event.TypedCreateEvent[*api.Host]) bool {
	libref.Mapper.Create(event.CreateEvent{Object: e.Object})
	return true
}

func (r HostPredicate) Update(e event.TypedUpdateEvent[*api.Host]) bool {
	object := e.ObjectNew
	changed := object.Status.ObservedGeneration < object.Generation
	if changed {
		libref.Mapper.Update(event.UpdateEvent{
			ObjectOld: e.ObjectOld,
			ObjectNew: e.ObjectNew,
		})
	}

	return changed
}

func (r HostPredicate) Delete(e event.TypedDeleteEvent[*api.Host]) bool {
	libref.Mapper.Delete(event.DeleteEvent{Object: e.Object})
	return true
}

// Provider watch predicate.
// Also ensures an inventory watch is created and
// associated with the channel source.
type ProviderPredicate struct {
	handler.WatchManager
	predicate.TypedFuncs[*api.Provider]
	channel chan event.GenericEvent
	client  client.Client
}

// Provider created event.
func (r *ProviderPredicate) Create(e event.TypedCreateEvent[*api.Provider]) bool {
	p := e.Object
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}

// Provider updated event.
func (r *ProviderPredicate) Update(e event.TypedUpdateEvent[*api.Provider]) bool {
	p := e.ObjectNew
	reconciled := p.Status.ObservedGeneration == p.Generation
	if reconciled {
		r.ensureWatch(p)
		return true
	}

	return false
}

// Provider deleted event.
func (r *ProviderPredicate) Delete(e event.TypedDeleteEvent[*api.Provider]) bool {
	p := e.Object
	r.WatchManager.Deleted(p)
	return true
}

// Generic provider watch event.
func (r *ProviderPredicate) Generic(e event.TypedGenericEvent[*api.Provider]) bool {
	p := e.Object
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}

// Ensure there is a watch for the provider
// and inventory API kinds.
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
