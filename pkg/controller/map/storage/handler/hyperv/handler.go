package hyperv

import (
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
)

// HyperV uses a single SMB share, no separate storage entities to watch
type Handler struct {
	*handler.Handler
}

func (r *Handler) Watch(_ *handler.WatchManager) error {
	return nil
}
