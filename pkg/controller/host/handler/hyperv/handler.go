package hyperv

import (
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
)

// HyperV is single-host, no host-level operations needed
type Handler struct {
	*handler.Handler
}

func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	return
}
