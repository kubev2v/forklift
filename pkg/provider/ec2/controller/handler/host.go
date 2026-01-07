package handler

import (
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
)

// NoOpHostHandler is a no-op host handler for EC2.
type NoOpHostHandler struct{}

// Watch is a no-op for EC2.
func (r *NoOpHostHandler) Watch(watch *handler.WatchManager) (err error) {
	return
}
