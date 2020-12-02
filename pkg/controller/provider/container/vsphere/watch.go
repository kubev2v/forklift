package vsphere

import (
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
)

//
// VM Watch.
// Watch for VM changes and analyze as needed.
type WatchVM struct {
	libmodel.DB
}

//
// VM Created.
// Analyzed as needed.
func (r WatchVM) Created(event libmodel.Event) {
	vm, cast := event.Model.(*model.VM)
	if !cast {
		return
	}
	if vm.Analyzed() {
		return
	}

	r.analyze(vm)
}

//
// VM Updated.
// Analyzed as needed.
func (r WatchVM) Updated(event libmodel.Event) {
	vm, cast := event.Updated.(*model.VM)
	if !cast {
		return
	}
	if vm.Analyzed() {
		return
	}

	r.analyze(vm)
}

// VM Deleted.
func (r WatchVM) Deleted(event libmodel.Event) {
}

//
// Report errors.
func (r WatchVM) Error(err error) {
	Log.Trace(liberr.Wrap(err))
}

//
// Watch ended.
func (r WatchVM) End() {
}

//
// Analyze the VM.
func (r WatchVM) analyze(vm *model.VM) {
	tx, err := r.DB.Begin()
	if err != nil {
		Log.Trace(liberr.Wrap(err))
		return
	}
	defer tx.End()
	err = tx.Get(vm)
	if err != nil {
		Log.Trace(liberr.Wrap(err))
		return
	}

	// TODO: Open Policy Agent - HERE

	vm.RevisionAnalyzed = vm.Revision
	err = tx.Update(vm)
	if err != nil {
		Log.Trace(liberr.Wrap(err))
		return
	}
	err = tx.Commit()
	if err != nil {
		Log.Trace(liberr.Wrap(err))
		return
	}
}

//
// Cluster Watch.
// Watch for cluster changes and analyze as needed.
type WatchCluster struct {
	libmodel.DB
}

//
// Cluster created.
// Analyze all related VMs.
func (r WatchCluster) Created(event libmodel.Event) {
	cluster, cast := event.Model.(*model.Cluster)
	if cast {
		r.analyze(cluster)
	}
}

//
// Cluster updated.
// Analyze all related VMs.
func (r WatchCluster) Updated(event libmodel.Event) {
	cluster, cast := event.Model.(*model.Cluster)
	if cast {
		r.analyze(cluster)
	}
}

//
// Cluster deleted.
func (r WatchCluster) Deleted(event libmodel.Event) {
}

//
// Report errors.
func (r WatchCluster) Error(err error) {
	Log.Trace(liberr.Wrap(err))
}

//
// Watch ended.
func (r WatchCluster) End() {
}

//
// Analyze all of the VMs related to the cluster.
func (r WatchCluster) analyze(cluster *model.Cluster) {
	tx, err := r.DB.Begin()
	if err != nil {
		Log.Trace(liberr.Wrap(err))
		return
	}
	defer tx.End()
	hostList := model.RefList{}
	hostList.With(cluster.Hosts)
	for _, ref := range hostList {
		host := &model.Host{}
		host.WithRef(ref)
		err = tx.Get(host)
		if err != nil {
			Log.Trace(liberr.Wrap(err))
			return
		}
		vmList := model.RefList{}
		vmList.With(host.Vms)
		for _, ref := range vmList {
			vm := &model.VM{}
			vm.WithRef(ref)
			err = tx.Get(vm)
			if err != nil {
				Log.Trace(liberr.Wrap(err))
				return
			}
			vm.RevisionAnalyzed = 0
			err = tx.Update(vm)
			if err != nil {
				Log.Trace(liberr.Wrap(err))
				return
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		Log.Trace(liberr.Wrap(err))
		return
	}
}
