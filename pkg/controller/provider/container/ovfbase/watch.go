package ovfbase

import (
	"context"
	"errors"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	refapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovf"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ovfbase"
	"github.com/kubev2v/forklift/pkg/controller/validation/policy"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
)

const (
	// The (max) number of batched task results.
	MaxBatch = 1024
	// Transaction label.
	ValidationLabel = "VM-validated"
)

// Endpoints.
const (
	BaseEndpoint       = "/v1/data/io/konveyor/forklift/ova/"
	VersionEndpoint    = BaseEndpoint + "rules_version"
	ValidationEndpoint = BaseEndpoint + "validate"
)

// Application settings.
var Settings = &settings.Settings

// Watch for VM changes and validate as needed.
type VMEventHandler struct {
	libmodel.StockEventHandler
	// Provider.
	Provider *api.Provider
	// DB.
	DB libmodel.DB
	// Validation event latch.
	latch chan int8
	// Last search.
	lastSearch time.Time
	// Logger.
	log logging.LevelLogger
	// Context
	context context.Context
	// Context cancel.
	cancel context.CancelFunc
	// Task result
	taskResult chan *policy.Task
}

// Reset.
func (r *VMEventHandler) reset() {
	r.lastSearch = time.Now()
}

// Watch ended.
func (r *VMEventHandler) Started(uint64) {
	r.log.Info("Started.")
	r.taskResult = make(chan *policy.Task)
	r.latch = make(chan int8, 1)
	r.context, r.cancel = context.WithCancel(context.Background())
	go r.run()
	go r.harvest()
}

// VM Created.
// The VM is scheduled (and reported as scheduled).
// This is best-effort.  If the validate() fails, it wil be
// picked up in the next search().
func (r *VMEventHandler) Created(event libmodel.Event) {
	if r.canceled() {
		return
	}
	if VM, cast := event.Model.(*model.VM); cast {
		if !VM.Validated() {
			r.tripLatch()
		}
	}
}

// VM Updated.
// The VM is scheduled (and reported as scheduled).
// This is best-effort.  If the validate() fails, it wil be
// picked up in the next search().
func (r *VMEventHandler) Updated(event libmodel.Event) {
	if r.canceled() {
		return
	}
	if event.HasLabel(ValidationLabel) {
		return
	}
	if VM, cast := event.Updated.(*model.VM); cast {
		if !VM.Validated() {
			r.tripLatch()
		}
	}
}

// Report errors.
func (r *VMEventHandler) Error(err error) {
	r.log.Error(liberr.Wrap(err), err.Error())
}

// Watch ended.
func (r *VMEventHandler) End() {
	r.log.Info("Ended.")
	r.cancel()
	close(r.latch)
	close(r.taskResult)
}

// Trip the validation event latch.
func (r *VMEventHandler) tripLatch() {
	defer func() {
		_ = recover()
	}()
	select {
	case r.latch <- 1:
		// trip.
	default:
		// tripped.
	}
}

// Run.
// Periodically search for VMs that need to be validated.
func (r *VMEventHandler) run() {
	r.log.Info("Run started.")
	defer r.log.Info("Run stopped.")
	interval := time.Second * time.Duration(
		Settings.PolicyAgent.SearchInterval)
	r.list()
	r.reset()
	for {
		select {
		case <-time.After(interval):
			r.list()
			r.reset()
		case _, open := <-r.latch:
			if open {
				r.list()
				r.reset()
			} else {
				return
			}
		}
	}
}

// Harvest validation task results and update VMs.
// Collect completed tasks in batches. Apply the batch
// to VMs when one of:
//   - The batch is full.
//   - No tasks have been received within
//     the delay period.
func (r *VMEventHandler) harvest() {
	r.log.Info("Harvest started.")
	defer r.log.Info("Harvest stopped.")
	long := time.Hour
	short := time.Second
	delay := long
	batch := []*policy.Task{}
	mark := time.Now()
	for {
		select {
		case <-time.After(delay):
		case task, open := <-r.taskResult:
			if open {
				batch = append(batch, task)
				delay = short
			} else {
				return
			}
		}
		if time.Since(mark) > delay || len(batch) > MaxBatch {
			r.validated(batch)
			batch = []*policy.Task{}
			delay = long
			mark = time.Now()
		}
	}
}

// List for VMs to be validated.
// VMs that have been reported through the model event
// watch are ignored.
func (r *VMEventHandler) list() {
	r.log.V(3).Info("List VMs that need to be validated.")
	version, err := policy.Agent.Version(VersionEndpoint)
	if err != nil {
		r.log.Error(err, err.Error())
		return
	}
	if r.canceled() {
		return
	}
	itr, err := r.DB.Find(
		&model.VM{},
		libmodel.ListOptions{
			Predicate: libmodel.Or(
				libmodel.Neq("Revision", libmodel.Field{Name: "RevisionValidated"}),
				libmodel.Neq("PolicyVersion", version)),
		})
	if err != nil {
		r.log.Error(err, "List VM failed.")
		return
	}
	if itr.Len() > 0 {
		r.log.V(3).Info(
			"List (unvalidated) VMs found.",
			"count",
			itr.Len())
	}
	for {
		VM := &model.VM{}
		hasNext := itr.NextWith(VM)
		if !hasNext || r.canceled() {
			break
		}
		_ = r.validate(VM)
	}
}

// Handler canceled.
func (r *VMEventHandler) canceled() bool {
	select {
	case <-r.context.Done():
		return true
	default:
		return false
	}
}

// Analyze the VM.
func (r *VMEventHandler) validate(VM *model.VM) (err error) {
	task := &policy.Task{
		Path:     ValidationEndpoint,
		Context:  r.context,
		Workload: r.workload,
		Result:   r.taskResult,
		Revision: VM.Revision,
		Ref: refapi.Ref{
			ID: VM.ID,
		},
	}
	r.log.V(4).Info(
		"Validate VM.",
		"VMID",
		VM.ID)
	err = policy.Agent.Submit(task)
	if err != nil {
		r.log.Error(err, "VM task (submit) failed.")
	}

	return
}

// VMs validated.
func (r *VMEventHandler) validated(batch []*policy.Task) {
	if len(batch) == 0 {
		return
	}
	r.log.V(3).Info(
		"VM (batch) completed.",
		"count",
		len(batch))
	tx, err := r.DB.Begin(ValidationLabel)
	if err != nil {
		r.log.Error(err, "Begin tx failed.")
		return
	}
	defer func() {
		_ = tx.End()
	}()
	for _, task := range batch {
		if task.Error != nil {
			r.log.Error(
				task.Error, "VM validation failed.")

			if len(task.Concerns) == 0 {
				continue
			}
			// If there are concerns we need to update and commit the changes
		}
		latest := &model.VM{Base: model.Base{ID: task.Ref.ID}}
		err = tx.Get(latest)
		if err != nil {
			r.log.Error(err, "VM (get) failed.")
			continue
		}
		if task.Revision != latest.Revision {
			continue
		}
		latest.PolicyVersion = task.Version
		latest.RevisionValidated = task.Revision
		latest.Concerns = task.Concerns
		latest.Revision--
		err = tx.Update(latest, libmodel.Eq("Revision", task.Revision))
		if errors.Is(err, model.NotFound) {
			continue
		}
		if err != nil {
			r.log.Error(err, "VM update failed.")
			continue
		}
		if task.Error == nil {
			r.log.V(3).Info(
				"VM validated.",
				"vmID",
				latest.ID,
				"revision",
				latest.Revision,
				"duration",
				task.Duration())
		}
	}
	err = tx.Commit()
	if err != nil {
		r.log.Error(err, "Tx commit failed.")
		return
	}
}

// Build the workload.
func (r *VMEventHandler) workload(vmID string) (object interface{}, err error) {
	vm := &model.VM{
		Base: model.Base{ID: vmID},
	}
	err = r.DB.Get(vm)
	if err != nil {
		return
	}
	workload := ovfbase.Workload{}
	workload.With(vm)
	err = workload.Expand(r.DB)
	if err != nil {
		return
	}

	workload.Link(r.Provider)
	object = workload

	return
}
