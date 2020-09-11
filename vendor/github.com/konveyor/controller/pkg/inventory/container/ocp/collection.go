package ocp

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

//
// Resource collection.
type Collection interface {
	predicate.Predicate
	// Bind to a reconciler.
	Bind(*Reconciler)
	// Get kubernetes resource object.
	Object() runtime.Object
	// Initial reconcile.
	Reconcile(context.Context) error
}

//
// Base collection.
type BaseCollection struct {
	// Associated data reconciler.
	Reconciler *Reconciler
}

//
// Associate with a reconciler.
func (r *BaseCollection) Bind(reconciler *Reconciler) {
	r.Reconciler = reconciler
}
