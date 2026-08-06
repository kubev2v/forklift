package migrator

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	migbase "github.com/kubev2v/forklift/pkg/controller/plan/migrator/base"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var _ migbase.Migrator = &Migrator{}

type Migrator struct {
	*plancontext.Context
	log logging.LevelLogger
}

func New(ctx *plancontext.Context) (migbase.Migrator, error) {
	return &Migrator{
		Context: ctx,
		log:     logging.WithName("migrator|azure"),
	}, nil
}

func (r *Migrator) Type() api.MigrationType       { return api.MigrationCold }
func (r *Migrator) Logger() logging.LevelLogger   { return r.log }
func (r *Migrator) Init() error                   { return nil }
func (r *Migrator) Begin() error                  { return nil }
func (r *Migrator) Complete(vm *planapi.VMStatus) {}
func (r *Migrator) Status(vm planapi.VM) *planapi.VMStatus {
	return &planapi.VMStatus{VM: vm}
}
func (r *Migrator) Reset(vm *planapi.VMStatus, pipeline []*planapi.Step) {
	vm.Pipeline = pipeline
	vm.Phase = api.PhaseStarted
	vm.Error = nil
	vm.Started = nil
	vm.Completed = nil
}

func (r *Migrator) Itinerary(vm planapi.VM) *libitr.Itinerary {
	return &libitr.Itinerary{
		Name: "Azure Cold Migration",
		Pipeline: libitr.Pipeline{
			{Name: api.PhaseStarted},
			{Name: api.PhaseCompleted},
		},
	}
}

func (r *Migrator) Pipeline(vm planapi.VM) ([]*planapi.Step, error) {
	return nil, nil
}

func (r *Migrator) Step(status *planapi.VMStatus) string {
	return ""
}

func (r *Migrator) ExecutePhase(vm *planapi.VMStatus) (bool, error) {
	return false, nil
}
