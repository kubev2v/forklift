package ocp

import (
	"github.com/konveyor/forklift-controller/pkg/controller/watch/handler"
)

//
// Provider watch event handler.
type Handler struct {
	*handler.Handler
}

//
// Ensure watch on hosts.
func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	return
}
