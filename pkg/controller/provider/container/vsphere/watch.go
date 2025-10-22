// The approach for providing VM policy-based integration has the
// following design constraints:
//   - Validation must never block updating the data model.
//   - Real-time validation is best effort.
//   - A scheduled search for VMs that needs to be validated
//     ensures that all VMs eventually get validated.
//
// Real-time validation is triggered by VM create/update model events.
// If the validation service is unavailable or fails, the condition
// is only logged with the intent that the next scheduled search will
// validate the latest version of VM.
// The scheduled search is a goroutine that periodically queries the
// DB for VMs with: revision != revisionValidated.  Each matched VM
// is validated.  To reduce overlap between the scheduled validation
// and event-driven validation, Each model event is "reported" (though
// a channel) to the search (loop). Reported are omitted from the search result.
// Both Cluster and Host model events result in all of the VMs in their respective
// containment trees will be updated with: revisionValidated = 0 which triggers
// (re)validation.
package vsphere

import (
	"context"
	"errors"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	refapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	web "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
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
	BaseEndpoint       = "/v1/data/io/konveyor/forklift/vmware/"
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
	if vm, cast := event.Model.(*model.VM); cast {
		if !vm.Validated() {
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
	if vm, cast := event.Updated.(*model.VM); cast {
		if !vm.Validated() {
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

// Trip the latch.
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
		vm := &model.VM{}
		hasNext := itr.NextWith(vm)
		if !hasNext || r.canceled() {
			break
		}
		_ = r.validate(vm)
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
func (r *VMEventHandler) validate(vm *model.VM) (err error) {
	task := &policy.Task{
		Path:     ValidationEndpoint,
		Context:  r.context,
		Workload: r.workload,
		Result:   r.taskResult,
		Revision: vm.Revision,
		Ref: refapi.Ref{
			ID: vm.ID,
		},
	}
	r.log.V(4).Info(
		"Validate VM.",
		"vmID",
		vm.ID)
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
	workload := web.Workload{}
	workload.With(vm)
	err = workload.Expand(r.DB)
	if err != nil {
		return
	}

	workload.Link(r.Provider)
	object = workload

	return
}

// Watch for cluster changes and validate as needed.
type ClusterEventHandler struct {
	libmodel.StockEventHandler
	// DB.
	DB libmodel.DB
	// Logger.
	log logging.LevelLogger
}

// Cluster updated.
// Analyze all related VMs.
func (r *ClusterEventHandler) Updated(event libmodel.Event) {
	cluster, cast := event.Model.(*model.Cluster)
	if cast {
		r.validate(cluster)
	}
}

// Report errors.
func (r *ClusterEventHandler) Error(err error) {
	r.log.Error(liberr.Wrap(err), err.Error())
}

// Analyze all of the VMs related to the cluster.
func (r *ClusterEventHandler) validate(cluster *model.Cluster) {
	for _, ref := range cluster.Hosts {
		host := &model.Host{}
		host.WithRef(ref)
		err := r.DB.Get(host)
		if err != nil {
			r.log.Error(err, "Host (get) failed.")
			return
		}
		hostHandler := HostEventHandler{DB: r.DB}
		hostHandler.validate(host)
	}
}

// Watch for host changes and validate as needed.
type HostEventHandler struct {
	libmodel.StockEventHandler
	// DB.
	DB libmodel.DB
	// Logger.
	log logging.LevelLogger
}

// Host updated.
// Analyze all related VMs.
func (r *HostEventHandler) Updated(event libmodel.Event) {
	host, cast := event.Model.(*model.Host)
	if cast {
		r.validate(host)
	}
}

// Report errors.
func (r *HostEventHandler) Error(err error) {
	r.log.Error(liberr.Wrap(err), err.Error())
}

// Analyze all of the VMs related to the host.
func (r *HostEventHandler) validate(host *model.Host) {
	tx, err := r.DB.Begin()
	if err != nil {
		r.log.Error(err, "begin tx failed.")
		return
	}
	defer func() {
		_ = tx.End()
	}()
	list := []model.VM{}
	err = tx.List(
		&list,
		model.ListOptions{
			Detail: model.MaxDetail,
			Predicate: libmodel.Eq(
				"host",
				host.ID),
		})
	if err != nil {
		r.log.Error(err, "VM (list) failed.")
		return
	}
	for _, vm := range list {
		vm.RevisionValidated = 0
		err = tx.Update(&vm)
		if err != nil {
			r.log.Error(err, "VM (update) failed.")
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		r.log.Error(err, "Tx commit failed.")
		return
	}
}
