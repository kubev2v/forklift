//
// The approach for providing VM policy-based integration has the
// following design constraints:
//   - Validation must never block updating the data model.
//   - Real-time validation is best effort.
//   - A scheduled search for VMs that needs to be validated
//     ensures that all VMs eventually get validated.
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
//
package vsphere

import (
	"errors"
	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	refapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/validation/policy"
	"github.com/konveyor/forklift-controller/pkg/settings"
	"time"
)

//
// Application settings.
var Settings = &settings.Settings

//
// Reported model event.
type ReportedEvent struct {
	// VM id.
	id string
	// VM revision.
	revision int64
}

//
// Watch for VM changes and validate as needed.
type VMEventHandler struct {
	libmodel.StockEventHandler
	// Provider.
	Provider *api.Provider
	// DB.
	DB libmodel.DB
	// Policy agent.
	policyAgent *policy.Scheduler
	// Reported VM events.
	input chan ReportedEvent
	// Reported VM IDs.
	reported map[string]int64
	// Last search.
	lastSearch time.Time
	// Logger.
	log logr.Logger
}

//
// Search interval.
func (r *VMEventHandler) searchInterval() time.Duration {
	seconds := Settings.PolicyAgent.SearchInterval
	if seconds < 60 {
		seconds = 60
	}

	return time.Second * time.Duration(seconds)
}

//
// Reset.
func (r *VMEventHandler) reset() {
	r.reported = map[string]int64{}
	r.lastSearch = time.Now()
}

//
// Watch ended.
func (r *VMEventHandler) Started(uint64) {
	r.input = make(chan ReportedEvent)
	r.policyAgent = policy.New(r.Provider)
	if !Settings.PolicyAgent.Enabled() {
		r.log.Info("Policy agent not enabled.")
		return
	}
	r.policyAgent.Start()
	go r.run()
}

//
// VM Created.
func (r *VMEventHandler) Created(event libmodel.Event) {
	if vm, cast := event.Model.(*model.VM); cast {
		if !vm.Validated() {
			r.report(vm)
			r.validate(vm)
		}
	}
}

//
// VM Updated.
func (r *VMEventHandler) Updated(event libmodel.Event) {
	if vm, cast := event.Updated.(*model.VM); cast {
		if !vm.Validated() {
			r.report(vm)
			r.validate(vm)
		}
	}
}

//
// Report errors.
func (r *VMEventHandler) Error(err error) {
	r.log.Error(liberr.Wrap(err), err.Error())
}

//
// Watch ended.
func (r *VMEventHandler) End() {
	r.policyAgent.Shutdown()
	close(r.input)
}

//
// Report model event.
func (r *VMEventHandler) report(vm *model.VM) {
	if !r.policyAgent.Enabled() {
		return
	}
	defer func() {
		_ = recover()
	}()
	r.input <- ReportedEvent{
		revision: vm.Revision,
		id:       vm.ID,
	}
}

//
// Run.
// Periodically search for VMs that need to be validated.
func (r *VMEventHandler) run() {
	interval := r.searchInterval()
	if !r.policyAgent.Enabled() {
		return
	}
	r.reset()
	for {
		select {
		case <-time.After(interval):
		case reportedEvent, open := <-r.input:
			if open {
				r.reported[reportedEvent.id] = reportedEvent.revision
			} else {
				return
			}
		}
		if time.Since(r.lastSearch) > interval {
			r.search()
			r.reset()
		}
	}
}

//
// Search for VMs to be validated.
// VMs that have been reported through the model event
// watch are ignored.
func (r *VMEventHandler) search() {
	r.log.V(1).Info("Search for VMs that need to be validated.")
	version, err := r.policyAgent.Version()
	if err != nil {
		r.log.Error(err, err.Error())
		return
	}
	for {
		list := []model.VM{}
		err = r.DB.List(
			&list,
			libmodel.ListOptions{
				Predicate: libmodel.Or(
					libmodel.Neq("Revision", libmodel.Field{Name: "RevisionValidated"}),
					libmodel.Neq("PolicyVersion", version)),
				Page: &libmodel.Page{
					Limit: 250,
				},
			})
		if err != nil {
			r.log.Error(err, "list VM failed.")
			return
		}
		if len(list) == 0 {
			return
		}
		for _, vm := range list {
			if revision, found := r.reported[vm.ID]; found {
				if vm.Revision == revision {
					continue
				}
			}
			r.validate(&vm)
		}
	}
}

//
// Analyze the VM.
func (r *VMEventHandler) validate(vm *model.VM) {
	var err error
	if !Settings.PolicyAgent.Enabled() {
		return
	}
	task := &policy.Task{
		ResultHandler: r.validated,
		Revision:      vm.Revision,
		Ref: refapi.Ref{
			ID: vm.ID,
		},
	}
	err = r.policyAgent.Submit(task)
	if err != nil {
		if errors.As(err, &policy.BacklogExceededError{}) {
			r.log.Info(err.Error())
		} else {
			r.log.Error(err, "submit failed.")
		}
	}
}

//
// VM validated.
func (r *VMEventHandler) validated(task *policy.Task) {
	if task.Error != nil {
		r.log.Info(task.Error.Error())
		return
	}
	tx, err := r.DB.Begin()
	if err != nil {
		r.log.Error(err, "begin tx failed.")
		return
	}
	defer func() {
		_ = tx.End()
	}()
	latest := &model.VM{Base: model.Base{ID: task.Ref.ID}}
	err = r.DB.Get(latest)
	if err != nil {
		r.log.Error(err, "get vm failed.")
		return
	}
	if task.Revision != latest.Revision {
		return
	}
	latest.PolicyVersion = task.Version
	latest.RevisionValidated = latest.Revision
	latest.Concerns = task.Concerns
	err = tx.Update(latest)
	if err != nil {
		r.log.Error(err, "update VM failed.")
		return
	}
	err = tx.Commit()
	if err != nil {
		r.log.Error(err, "commit failed.")
		return
	}
	r.log.V(3).Info(
		"PolicyAgent: validated",
		"vmID",
		latest.ID,
		"revision",
		latest.Revision,
		"duration",
		task.Duration())
}

//
// Watch for cluster changes and validate as needed.
type ClusterEventHandler struct {
	libmodel.StockEventHandler
	// DB.
	DB libmodel.DB
	// Logger.
	log logr.Logger
}

//
// Cluster updated.
// Analyze all related VMs.
func (r *ClusterEventHandler) Updated(event libmodel.Event) {
	cluster, cast := event.Model.(*model.Cluster)
	if cast {
		r.validate(cluster)
	}
}

//
// Report errors.
func (r *ClusterEventHandler) Error(err error) {
	r.log.Error(liberr.Wrap(err), err.Error())
}

//
// Analyze all of the VMs related to the cluster.
func (r *ClusterEventHandler) validate(cluster *model.Cluster) {
	if !Settings.PolicyAgent.Enabled() {
		return
	}
	for _, ref := range cluster.Hosts {
		host := &model.Host{}
		host.WithRef(ref)
		err := r.DB.Get(host)
		if err != nil {
			r.log.Error(err, "get host failed.")
			return
		}
		hostHandler := HostEventHandler{DB: r.DB}
		hostHandler.validate(host)
	}
}

//
// Watch for host changes and validate as needed.
type HostEventHandler struct {
	libmodel.StockEventHandler
	// DB.
	DB libmodel.DB
	// Logger.
	log logr.Logger
}

//
// Host updated.
// Analyze all related VMs.
func (r *HostEventHandler) Updated(event libmodel.Event) {
	host, cast := event.Model.(*model.Host)
	if cast {
		r.validate(host)
	}
}

//
// Report errors.
func (r *HostEventHandler) Error(err error) {
	r.log.Error(liberr.Wrap(err), err.Error())
}

//
// Analyze all of the VMs related to the host.
func (r *HostEventHandler) validate(host *model.Host) {
	if !Settings.PolicyAgent.Enabled() {
		return
	}
	tx, err := r.DB.Begin()
	if err != nil {
		r.log.Error(err, "begin tx failed.")
		return
	}
	defer func() {
		_ = tx.End()
	}()
	for _, ref := range host.Vms {
		vm := &model.VM{}
		vm.WithRef(ref)
		err = tx.Get(vm)
		if err != nil {
			r.log.Error(err, "get VM failed.")
			return
		}
		vm.RevisionValidated = 0
		err = tx.Update(vm)
		if err != nil {
			r.log.Error(err, "update VM failed.")
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		r.log.Error(err, "tx commit failed.")
		return
	}
}
