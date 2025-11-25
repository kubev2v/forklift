package migrator

import (
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/migrator/base"
	"github.com/kubev2v/forklift/pkg/controller/plan/migrator/ocp"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ec2migrator "github.com/kubev2v/forklift/pkg/provider/ec2/controller/migrator"
)

type Migrator = base.Migrator

var log = logging.WithName("migrator")

// New builds a new Migrator implementation from a plan context.
func New(context *plancontext.Context) (migrator Migrator, err error) {
	switch context.Source.Provider.Type() {
	case api.OpenShift:
		migrator, err = ocp.New(context)
		if err != nil {
			return
		}
	case api.EC2:
		migrator, err = ec2migrator.New(context)
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

// NextPhase transitions the VM to the next migration phase.
// If this was the last phase in the current pipeline step, the pipeline step
// is marked complete. Alias of base.NextPhase.
func NextPhase(m Migrator, vm *planapi.VMStatus) {
	base.NextPhase(m, vm)
}
