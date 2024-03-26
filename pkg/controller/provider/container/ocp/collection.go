package ocp

import (
	"context"

	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libocp "github.com/konveyor/forklift-controller/pkg/lib/inventory/container/ocp"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	libref "github.com/konveyor/forklift-controller/pkg/lib/ref"
	core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	cnv "kubevirt.io/api/core/v1"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// StorageClass
type StorageClass struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *StorageClass) Object() client.Object {
	return &storage.StorageClass{}
}

// Reconcile.
// Achieve initial consistency.
func (r *StorageClass) Reconcile(ctx context.Context) (err error) {
	pClient := r.Collector.Client()
	list := &storage.StorageClassList{}
	err = pClient.List(context.TODO(), list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	db := r.Collector.DB()
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
		r.Collector.UpdateThreshold(m)
		r.log.Info("Create", libref.ToKind(m), m.String())
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

// Resource created watch event.
func (r *StorageClass) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*storage.StorageClass)
	if !cast {
		return false
	}
	m := &model.StorageClass{}
	m.With(object)
	r.Collector.Create(m)

	return false
}

// Resource updated watch event.
func (r *StorageClass) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*storage.StorageClass)
	if !cast {
		return false
	}
	m := &model.StorageClass{}
	m.With(object)
	r.Collector.Update(m)

	return false
}

// Resource deleted watch event.
func (r *StorageClass) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*storage.StorageClass)
	if !cast {
		return false
	}
	m := &model.StorageClass{}
	m.With(object)
	r.Collector.Delete(m)

	return false
}

// Ignored.
func (r *StorageClass) Generic(e event.GenericEvent) bool {
	return false
}

// NetworkAttachmentDefinition
type NetworkAttachmentDefinition struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *NetworkAttachmentDefinition) Object() client.Object {
	return &net.NetworkAttachmentDefinition{}
}

// Reconcile.
// Achieve initial consistency.
func (r *NetworkAttachmentDefinition) Reconcile(ctx context.Context) (err error) {
	pClient := r.Collector.Client()
	list := &net.NetworkAttachmentDefinitionList{}
	err = pClient.List(context.TODO(), list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	db := r.Collector.DB()
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
		r.Collector.UpdateThreshold(m)
		r.log.Info("Create", libref.ToKind(m), m.String())
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

// Resource created watch event.
func (r *NetworkAttachmentDefinition) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*net.NetworkAttachmentDefinition)
	if !cast {
		return false
	}
	m := &model.NetworkAttachmentDefinition{}
	m.With(object)
	r.Collector.Create(m)

	return false
}

// Resource updated watch event.
func (r *NetworkAttachmentDefinition) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*net.NetworkAttachmentDefinition)
	if !cast {
		return false
	}
	m := &model.NetworkAttachmentDefinition{}
	m.With(object)
	r.Collector.Update(m)

	return false
}

// Resource deleted watch event.
func (r *NetworkAttachmentDefinition) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*net.NetworkAttachmentDefinition)
	if !cast {
		return false
	}
	m := &model.NetworkAttachmentDefinition{}
	m.With(object)
	r.Collector.Delete(m)

	return false
}

// Ignored.
func (r *NetworkAttachmentDefinition) Generic(e event.GenericEvent) bool {
	return false
}

// Namespace
type Namespace struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *Namespace) Object() client.Object {
	return &core.Namespace{}
}

// Reconcile.
// Achieve initial consistency.
func (r *Namespace) Reconcile(ctx context.Context) (err error) {
	pClient := r.Collector.Client()
	list := &core.NamespaceList{}
	err = pClient.List(context.TODO(), list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	db := r.Collector.DB()
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
		r.Collector.UpdateThreshold(m)
		r.log.Info("Create", libref.ToKind(m), m.String())
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

// Resource created watch event.
func (r *Namespace) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*core.Namespace)
	if !cast {
		return false
	}
	m := &model.Namespace{}
	m.With(object)
	r.Collector.Create(m)

	return false
}

// Resource updated watch event.
func (r *Namespace) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*core.Namespace)
	if !cast {
		return false
	}
	m := &model.Namespace{}
	m.With(object)
	r.Collector.Update(m)

	return false
}

// Resource deleted watch event.
func (r *Namespace) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*core.Namespace)
	if !cast {
		return false
	}
	m := &model.Namespace{}
	m.With(object)
	r.Collector.Delete(m)

	return false
}

// Ignored.
func (r *Namespace) Generic(e event.GenericEvent) bool {
	return false
}

// VM
type VM struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *VM) Object() client.Object {
	return &cnv.VirtualMachine{}
}

// Reconcile.
// Achieve initial consistency.
func (r *VM) Reconcile(ctx context.Context) (err error) {
	pClient := r.Collector.Client()
	list := &cnv.VirtualMachineList{}
	err = pClient.List(context.TODO(), list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	db := r.Collector.DB()
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
		r.Collector.UpdateThreshold(m)
		r.log.Info("Create", libref.ToKind(m), m.String())
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

// Resource created watch event.
func (r *VM) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*cnv.VirtualMachine)
	if !cast {
		return false
	}
	m := &model.VM{}
	m.With(object)
	r.Collector.Create(m)

	return false
}

// Resource updated watch event.
func (r *VM) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*cnv.VirtualMachine)
	if !cast {
		return false
	}
	m := &model.VM{}
	m.With(object)
	r.Collector.Update(m)

	return false
}

// Resource deleted watch event.
func (r *VM) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*cnv.VirtualMachine)
	if !cast {
		return false
	}
	m := &model.VM{}
	m.With(object)
	r.Collector.Delete(m)

	return false
}

// Ignored.
func (r *VM) Generic(e event.GenericEvent) bool {
	return false
}

// InstanceType
type InstanceType struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *InstanceType) Object() client.Object {
	return &instancetype.VirtualMachineInstancetype{}
}

// Reconcile.
// Achieve initial consistency.
func (r *InstanceType) Reconcile(ctx context.Context) (err error) {
	pClient := r.Collector.Client()
	list := &instancetype.VirtualMachineInstancetypeList{}
	err = pClient.List(context.TODO(), list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	db := r.Collector.DB()
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
			m := &model.InstanceType{}
			m.With(&resource)
			r.Collector.UpdateThreshold(m)
			r.log.Info("Create", libref.ToKind(m), m.String())
			err = tx.Insert(m)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

// Resource created watch event.
func (r *InstanceType) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*instancetype.VirtualMachineInstancetype)
	if !cast {
		return false
	}
	m := &model.InstanceType{}
	m.With(object)
	r.Collector.Create(m)

	return false
}

// Resource updated watch event.
func (r *InstanceType) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*instancetype.VirtualMachineInstancetype)
	if !cast {
		return false
	}
	m := &model.InstanceType{}
	m.With(object)
	r.Collector.Update(m)

	return false
}

// Resource deleted watch event.
func (r *InstanceType) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*instancetype.VirtualMachineInstancetype)
	if !cast {
		return false
	}
	m := &model.InstanceType{}
	m.With(object)
	r.Collector.Delete(m)

	return false
}

// Ignored.
func (r *InstanceType) Generic(e event.GenericEvent) bool {
	return false
}

// ClusterInstanceType
type ClusterInstanceType struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *ClusterInstanceType) Object() client.Object {
	return &instancetype.VirtualMachineClusterInstancetype{}
}

// Reconcile.
// Achieve initial consistency.
func (r *ClusterInstanceType) Reconcile(ctx context.Context) (err error) {
	pClient := r.Collector.Client()
	list := &instancetype.VirtualMachineClusterInstancetypeList{}
	err = pClient.List(context.TODO(), list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	db := r.Collector.DB()
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
			m := &model.ClusterInstanceType{}
			m.With(&resource)
			r.Collector.UpdateThreshold(m)
			r.log.Info("Create", libref.ToKind(m), m.String())
			err = tx.Insert(m)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

// Resource created watch event.
func (r *ClusterInstanceType) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*instancetype.VirtualMachineClusterInstancetype)
	if !cast {
		return false
	}
	m := &model.ClusterInstanceType{}
	m.With(object)
	r.Collector.Create(m)

	return false
}

// Resource updated watch event.
func (r *ClusterInstanceType) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*instancetype.VirtualMachineClusterInstancetype)
	if !cast {
		return false
	}
	m := &model.ClusterInstanceType{}
	m.With(object)
	r.Collector.Update(m)

	return false
}

// Resource deleted watch event.
func (r *ClusterInstanceType) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*instancetype.VirtualMachineClusterInstancetype)
	if !cast {
		return false
	}
	m := &model.ClusterInstanceType{}
	m.With(object)
	r.Collector.Delete(m)

	return false
}

// Ignored.
func (r *ClusterInstanceType) Generic(e event.GenericEvent) bool {
	return false
}
