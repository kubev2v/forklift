package ocp

import (
	"context"
	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	liberr "github.com/konveyor/controller/pkg/error"
	libocp "github.com/konveyor/controller/pkg/inventory/container/ocp"
	libref "github.com/konveyor/controller/pkg/ref"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cnv "kubevirt.io/client-go/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

//
// StorageClass
type StorageClass struct {
	libocp.BaseCollection
}

//
// Get the kubernetes object being collected.
func (r *StorageClass) Object() runtime.Object {
	return &storage.StorageClass{}
}

//
// Reconcile.
// Achieve initial consistency.
func (r *StorageClass) Reconcile(ctx context.Context) (err error) {
	pClient := r.Reconciler.Client()
	list := &storage.StorageClassList{}
	err = pClient.List(context.TODO(), list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	db := r.Reconciler.DB()
	tx, err := db.Begin()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	defer tx.End()
	for _, resource := range list.Items {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		m := &model.StorageClass{}
		m.With(&resource)
		r.Reconciler.UpdateThreshold(m)
		Log.Info("Create", libref.ToKind(m), m.String())
		err = tx.Insert(m)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

//
// Resource created watch event.
func (r *StorageClass) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*storage.StorageClass)
	if !cast {
		return false
	}
	m := &model.StorageClass{}
	m.With(object)
	r.Reconciler.Create(m)

	return false
}

//
// Resource updated watch event.
func (r *StorageClass) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*storage.StorageClass)
	if !cast {
		return false
	}
	m := &model.StorageClass{}
	m.With(object)
	r.Reconciler.Update(m)

	return false
}

//
// Resource deleted watch event.
func (r *StorageClass) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*storage.StorageClass)
	if !cast {
		return false
	}
	m := &model.StorageClass{}
	m.With(object)
	r.Reconciler.Delete(m)

	return false
}

//
// Ignored.
func (r *StorageClass) Generic(e event.GenericEvent) bool {
	return false
}

//
// NetworkAttachmentDefinition
type NetworkAttachmentDefinition struct {
	libocp.BaseCollection
}

//
// Get the kubernetes object being collected.
func (r *NetworkAttachmentDefinition) Object() runtime.Object {
	return &net.NetworkAttachmentDefinition{}
}

//
// Reconcile.
// Achieve initial consistency.
func (r *NetworkAttachmentDefinition) Reconcile(ctx context.Context) (err error) {
	pClient := r.Reconciler.Client()
	list := &net.NetworkAttachmentDefinitionList{}
	err = pClient.List(context.TODO(), list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	db := r.Reconciler.DB()
	tx, err := db.Begin()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	defer tx.End()
	for _, resource := range list.Items {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		m := &model.NetworkAttachmentDefinition{}
		m.With(&resource)
		r.Reconciler.UpdateThreshold(m)
		Log.Info("Create", libref.ToKind(m), m.String())
		err = tx.Insert(m)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

//
// Resource created watch event.
func (r *NetworkAttachmentDefinition) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*net.NetworkAttachmentDefinition)
	if !cast {
		return false
	}
	m := &model.NetworkAttachmentDefinition{}
	m.With(object)
	r.Reconciler.Create(m)

	return false
}

//
// Resource updated watch event.
func (r *NetworkAttachmentDefinition) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*net.NetworkAttachmentDefinition)
	if !cast {
		return false
	}
	m := &model.NetworkAttachmentDefinition{}
	m.With(object)
	r.Reconciler.Update(m)

	return false
}

//
// Resource deleted watch event.
func (r *NetworkAttachmentDefinition) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*net.NetworkAttachmentDefinition)
	if !cast {
		return false
	}
	m := &model.NetworkAttachmentDefinition{}
	m.With(object)
	r.Reconciler.Delete(m)

	return false
}

//
// Ignored.
func (r *NetworkAttachmentDefinition) Generic(e event.GenericEvent) bool {
	return false
}

//
// Namespace
type Namespace struct {
	libocp.BaseCollection
}

//
// Get the kubernetes object being collected.
func (r *Namespace) Object() runtime.Object {
	return &storage.StorageClass{}
}

//
// Reconcile.
// Achieve initial consistency.
func (r *Namespace) Reconcile(ctx context.Context) (err error) {
	pClient := r.Reconciler.Client()
	list := &core.NamespaceList{}
	err = pClient.List(context.TODO(), list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	db := r.Reconciler.DB()
	tx, err := db.Begin()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	defer tx.End()
	for _, resource := range list.Items {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		m := &model.Namespace{}
		m.With(&resource)
		r.Reconciler.UpdateThreshold(m)
		Log.Info("Create", libref.ToKind(m), m.String())
		err = tx.Insert(m)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

//
// Resource created watch event.
func (r *Namespace) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*core.Namespace)
	if !cast {
		return false
	}
	m := &model.Namespace{}
	m.With(object)
	r.Reconciler.Create(m)

	return false
}

//
// Resource updated watch event.
func (r *Namespace) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*core.Namespace)
	if !cast {
		return false
	}
	m := &model.Namespace{}
	m.With(object)
	r.Reconciler.Update(m)

	return false
}

//
// Resource deleted watch event.
func (r *Namespace) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*core.Namespace)
	if !cast {
		return false
	}
	m := &model.Namespace{}
	m.With(object)
	r.Reconciler.Delete(m)

	return false
}

//
// Ignored.
func (r *Namespace) Generic(e event.GenericEvent) bool {
	return false
}

//
// VM
type VM struct {
	libocp.BaseCollection
}

//
// Get the kubernetes object being collected.
func (r *VM) Object() runtime.Object {
	return &cnv.VirtualMachine{}
}

//
// Reconcile.
// Achieve initial consistency.
func (r *VM) Reconcile(ctx context.Context) (err error) {
	pClient := r.Reconciler.Client()
	list := &cnv.VirtualMachineList{}
	err = pClient.List(context.TODO(), list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	db := r.Reconciler.DB()
	tx, err := db.Begin()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	defer tx.End()
	for _, resource := range list.Items {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		m := &model.VM{}
		m.With(&resource)
		r.Reconciler.UpdateThreshold(m)
		Log.Info("Create", libref.ToKind(m), m.String())
		err = tx.Insert(m)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

//
// Resource created watch event.
func (r *VM) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*cnv.VirtualMachine)
	if !cast {
		return false
	}
	m := &model.VM{}
	m.With(object)
	r.Reconciler.Create(m)

	return false
}

//
// Resource updated watch event.
func (r *VM) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*cnv.VirtualMachine)
	if !cast {
		return false
	}
	m := &model.VM{}
	m.With(object)
	r.Reconciler.Update(m)

	return false
}

//
// Resource deleted watch event.
func (r *VM) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*cnv.VirtualMachine)
	if !cast {
		return false
	}
	m := &model.VM{}
	m.With(object)
	r.Reconciler.Delete(m)

	return false
}

//
// Ignored.
func (r *VM) Generic(e event.GenericEvent) bool {
	return false
}
