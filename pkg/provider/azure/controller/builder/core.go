package builder

import (
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

type Builder struct {
	*plancontext.Context
	log logging.LevelLogger
}

func New(ctx *plancontext.Context) *Builder {
	log := logging.WithName("builder|azure")
	return &Builder{
		Context: ctx,
		log:     log,
	}
}
