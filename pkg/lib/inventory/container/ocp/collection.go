package ocp

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Resource collection.
type Collection interface {
	// Bind to a collector.
	Bind(*Collector)
	// Get kubernetes resource object.
	Object() client.Object
}

// Base collection.
type BaseCollection struct {
	// Associated data collector.
	Collector *Collector
}

// Associate with a collector.
func (r *BaseCollection) Bind(collector *Collector) {
	r.Collector = collector
}
