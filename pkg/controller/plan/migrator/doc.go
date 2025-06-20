package migrator

import (
	"path"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/migrator/base"
	"github.com/kubev2v/forklift/pkg/controller/plan/migrator/ocp"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

type Migrator = base.Migrator

var log = logging.WithName("migrator")

func New(context *plancontext.Context) (migrator Migrator, err error) {
	switch context.Source.Provider.Type() {
	case v1beta1.OpenShift:
		migrator, err = ocp.New(context)
		if err != nil {
			return
		}
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
