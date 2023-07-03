package ova

import (
	"github.com/konveyor/forklift-controller/pkg/controller/watch/handler"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
)

// Package logger.
var log = logging.WithName("storageMap|ova")

// Provider watch event handler.
type Handler struct {
	*handler.Handler
}
