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
	// Bind to a collector.
	Bind(*Collector)
	// Get kubernetes resource object.
	Object() runtime.Object
	// Initial reconcile.
	Reconcile(context.Context) error
}

//
// Base collection.
type BaseCollection struct {
	// Associated data collector.
	Collector *Collector
}

//
// Associate with a collector.
func (r *BaseCollection) Bind(collector *Collector) {
	r.Collector = collector
}
