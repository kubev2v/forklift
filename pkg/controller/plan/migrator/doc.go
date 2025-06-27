package migrator

import (
	"path"

	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/migrator/base"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

type Migrator = base.Migrator

var log = logging.WithName("migrator")

func New(context *plancontext.Context) (migrator Migrator, err error) {
	switch context.Source.Provider.Type() {
	default:
		m := base.BaseMigrator{Context: context}
		err = m.Init()
		if err != nil {
			return
		}
		migrator = &m
	}
	log.Info("Built migrator.", "plan", path.Join(context.Plan.Namespace, context.Plan.Name), "type", context.Source.Provider.Type())
	return
}
