package provider

import (
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ProviderPredicate struct {
	predicate.Funcs
}

func (r ProviderPredicate) Create(e event.CreateEvent) bool {
	_, cast := e.Object.(*api.Provider)
	if cast {
		libref.Mapper.Create(e)
		return true
	}

	return false
}

func (r ProviderPredicate) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*api.Provider)
	if !cast {
		return false
	}
	changed := object.Status.ObservedGeneration < object.Generation
	if changed {
		libref.Mapper.Update(e)
	}

	return changed
}

func (r ProviderPredicate) Delete(e event.DeleteEvent) bool {
	_, cast := e.Object.(*api.Provider)
	if cast {
		libref.Mapper.Delete(e)
		return true
	}

	return false
}
