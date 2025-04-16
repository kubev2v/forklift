package ocp

import (
	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	libocp "github.com/kubev2v/forklift/pkg/lib/inventory/container/ocp"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	cnv "kubevirt.io/api/core/v1"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// NetworkAttachmentDefinition
type NetworkAttachmentDefinition struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *NetworkAttachmentDefinition) Object() client.Object {
	return &net.NetworkAttachmentDefinition{}
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

// VM
type VM struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *VM) Object() client.Object {
	return &cnv.VirtualMachine{}
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
	defer func() {
		err = endTransaction(tx)
	}()
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

type PersistentVolumeClaim struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

func (r *PersistentVolumeClaim) Object() client.Object {
	return &core.PersistentVolumeClaim{}
}

func (r *PersistentVolumeClaim) Reconcile(ctx context.Context) (err error) {
	pClient := r.Collector.Client()
	list := &core.PersistentVolumeClaimList{}
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
	defer func() {
		err = endTransaction(tx)
	}()
	for _, resource := range list.Items {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		m := &model.PersistentVolumeClaim{}
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
func (r *PersistentVolumeClaim) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*core.PersistentVolumeClaim)
	if !cast {
		return false
	}
	m := &model.PersistentVolumeClaim{}
	m.With(object)
	r.Collector.Create(m)

	return false
}

// Resource updated watch event.
func (r *PersistentVolumeClaim) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*core.PersistentVolumeClaim)
	if !cast {
		return false
	}
	m := &model.PersistentVolumeClaim{}
	m.With(object)
	r.Collector.Update(m)

	return false
}

// Resource deleted watch event.
func (r *PersistentVolumeClaim) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*core.PersistentVolumeClaim)
	if !cast {
		return false
	}
	m := &model.PersistentVolumeClaim{}
	m.With(object)
	r.Collector.Delete(m)

	return false
}

// Ignored.
func (r *PersistentVolumeClaim) Generic(e event.GenericEvent) bool {
	return false
}

type DataVolume struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

func (r *DataVolume) Object() client.Object {
	return &cdi.DataVolume{}
}

func (r *DataVolume) Reconcile(ctx context.Context) (err error) {
	pClient := r.Collector.Client()
	list := &cdi.DataVolumeList{}
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
	defer func() {
		err = endTransaction(tx)
	}()
	for _, resource := range list.Items {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		m := &model.DataVolume{}
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
func (r *DataVolume) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*cdi.DataVolume)
	if !cast {
		return false
	}
	m := &model.DataVolume{}
	m.With(object)
	r.Collector.Create(m)

	return false
}

// Resource updated watch event.
func (r *DataVolume) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*cdi.DataVolume)
	if !cast {
		return false
	}
	m := &model.DataVolume{}
	m.With(object)
	r.Collector.Update(m)

	return false
}

// Resource deleted watch event.
func (r *DataVolume) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*cdi.DataVolume)
	if !cast {
		return false
	}
	m := &model.DataVolume{}
	m.With(object)
	r.Collector.Delete(m)

	return false
}

// Ignored.
func (r *DataVolume) Generic(e event.GenericEvent) bool {
	return false
}

func endTransaction(tx *m.Tx) error {
	err := tx.End()
	if err != nil {
		err = liberr.Wrap(err)
	}
	return err
}
