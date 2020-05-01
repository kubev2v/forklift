package plan

import (
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type PlanPredicate struct {
	predicate.Funcs
}

func (r PlanPredicate) Create(e event.CreateEvent) bool {
	_, cast := e.Object.(*api.Plan)
	if cast {
		libref.Mapper.Create(e)
		return true
	}

	return false
}

func (r PlanPredicate) Update(e event.UpdateEvent) bool {
	old, cast := e.ObjectOld.(*api.Plan)
	if !cast {
		return false
	}
	new, cast := e.ObjectNew.(*api.Plan)
	if !cast {
		return false
	}
	changed := !reflect.DeepEqual(old.Spec, new.Spec) ||
		!reflect.DeepEqual(
			old.DeletionTimestamp,
			new.DeletionTimestamp)
	if changed {
		libref.Mapper.Update(e)
	}
	return changed
}

func (r PlanPredicate) Delete(e event.DeleteEvent) bool {
	_, cast := e.Object.(*api.Plan)
	if cast {
		libref.Mapper.Delete(e)
		return true
	}

	return false
}
