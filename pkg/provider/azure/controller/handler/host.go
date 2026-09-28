package handler

import (
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
)

type NoOpHostHandler struct{}

func (r *NoOpHostHandler) Watch(watch *handler.WatchManager) (err error) {
	return
}
