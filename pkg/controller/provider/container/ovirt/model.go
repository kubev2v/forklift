package ovirt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovirt"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	fb "github.com/kubev2v/forklift/pkg/lib/filebacked"
	libcnt "github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

// Event codes.
const (
	// DataCenter
	USER_ADD_STORAGE_POOL    = 950
	USER_UPDATE_STORAGE_POOL = 952
	USER_REMOVE_STORAGE_POOL = 954
	// Network
	NETWORK_ADD_NETWORK    = 942
	NETWORK_UPDATE_NETWORK = 1114
	NETWORK_REMOVE_NETWORK = 944
	// Storage Domain
	USER_ADD_STORAGE_DOMAIN              = 956
	USER_UPDATE_STORAGE_DOMAIN           = 958
	USER_REMOVE_STORAGE_DOMAIN           = 960
	USER_FORCE_REMOVE_STORAGE_DOMAIN     = 981
	USER_DETACH_STORAGE_DOMAIN_FROM_POOL = 964
	// vNIC Profile
	ADD_VNIC_PROFILE    = 1122
	UPDATE_VNIC_PROFILE = 1124
	REMOVE_VNIC_PROFILE = 1126
	// Disk Profile
	USER_ADD_DISK_PROFILE    = 10120
	USER_UPDATE_DISK_PROFILE = 10124
	USER_REMOVE_DISK_PROFILE = 10122
	// Cluster
	USER_ADD_CLUSTER    = 809
	USER_UPDATE_CLUSTER = 811
	USER_REMOVE_CLUSTER = 813
	// Host
	USER_ADD_VDS                        = 42
	USER_UPDATE_VDS                     = 43
	USER_REMOVE_VDS                     = 44
	USER_VDS_MAINTENANCE                = 600
	USER_VDS_MAINTENANCE_WITHOUT_REASON = 620
	USER_VDS_MAINTENANCE_MANUAL_HA      = 10453
	VDS_DETECTED                        = 13
	// VM
	USER_ADD_VM                                    = 34
	USER_ADD_VM_FINISHED_SUCCESS                   = 53
	USER_UPDATE_VM                                 = 35
	SYSTEM_UPDATE_VM                               = 253
	USER_REMOVE_VM                                 = 113
	USER_REMOVE_VM_FINISHED_INTERNAL               = 1130
	USER_REMOVE_VM_FINISHED_ILLEGAL_DISKS          = 172
	USER_REMOVE_VM_FINISHED_ILLEGAL_DISKS_INTERNAL = 1720
	IMPORTEXPORT_IMPORT_VM                         = 1152
	USER_ADD_DISK_TO_VM_SUCCESS                    = 97
	USER_UPDATE_VM_DISK                            = 88
	USER_REMOVE_DISK_FROM_VM                       = 80
	USER_ATTACH_DISK_TO_VM                         = 2016
	USER_DETACH_DISK_FROM_VM                       = 2018
	USER_EJECT_VM_DISK                             = 521
	USER_CHANGE_DISK_VM                            = 38
	NETWORK_USER_ADD_VM_INTERFACE                  = 932
	NETWORK_USER_UPDATE_VM_INTERFACE               = 934
	NETWORK_USER_REMOVE_VM_INTERFACE               = 930
	USER_CREATE_SNAPSHOT_FINISHED_SUCCESS          = 68
	USER_REMOVE_SNAPSHOT_FINISHED_SUCCESS          = 356
	VM_ADD_HOST_DEVICES                            = 10800
	VM_REMOVE_HOST_DEVICES                         = 10801
	USER_RUN_VM                                    = 32
	USER_SUSPEND_VM_OK                             = 503
	USER_PAUSE_VM                                  = 39
	USER_RESUME_VM                                 = 40
	VM_DOWN                                        = 61
	// Disk
	USER_ADD_DISK_FINISHED_SUCCESS            = 2021
	USER_REMOVE_DISK                          = 2014
	USER_FINISHED_REMOVE_DISK_ATTACHED_TO_VMS = 2042
)

// All adapters.
var adapterList []Adapter

// Event (type) mapped to adapter.
var adapterMap = map[int][]Adapter{}

func init() {
	adapterList = []Adapter{
		&DataCenterAdapter{},
		&StorageDomainAdapter{},
		&NICProfileAdapter{},
		&DiskProfileAdapter{},
		&NetworkAdapter{},
		&DiskAdapter{},
		&ClusterAdapter{},
		&ServerCPUAdapter{},
		&HostAdapter{},
		&VMAdapter{},
	}
	for _, adapter := range adapterList {
		for _, event := range adapter.Event() {
			adapterMap[event] = append(
				adapterMap[event],
				adapter)
		}
	}
}

// Updates the DB based on
// changes described by an Event.
type Updater func(tx *libmodel.Tx) error

// Adapter context.
type Context struct {
	// Context.
	ctx context.Context
	// oVirt client.
	client *Client
	// Log.
	log logging.LevelLogger
}

// The adapter request is canceled.
func (r *Context) canceled() (done bool) {
	select {
	case <-r.ctx.Done():
		done = true
	default:
	}

	return
}

// Model adapter.
// Provides integration between the REST resource
// model and the inventory model.
type Adapter interface {
	// List REST collections.
	List(ctx *Context) (itr fb.Iterator, err error)
	// Apply an event to the inventory model.
	Apply(ctx *Context, event *Event) (updater Updater, err error)
	// List handled event (codes).
	Event() []int
}

// Base adapter.
type BaseAdapter struct {
}

// Build follow parameter.
func (r *BaseAdapter) follow(property ...string) libweb.Param {
	return libweb.Param{
		Key: "follow",
		Value: strings.Join(
			property,
			","),
	}
}

// DataCenter.
type DataCenterAdapter struct {
	BaseAdapter
}

// List the collection.
func (r *DataCenterAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	dataCenterList := DataCenterList{}
	err = ctx.client.list("datacenters", &dataCenterList)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range dataCenterList.Items {
		m := &model.DataCenter{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		list.Append(m)
	}

	itr = list.Iter()

	return
}

// Handled events.
func (r *DataCenterAdapter) Event() []int {
	return []int{
		USER_ADD_STORAGE_POOL,
		USER_UPDATE_STORAGE_POOL,
		USER_REMOVE_STORAGE_POOL,
	}
}

// Apply events to the inventory model.
func (r *DataCenterAdapter) Apply(ctx *Context, event *Event) (updater Updater, err error) {
	defer func() {
		if errors.Is(err, &NotFound{}) {
			updater = func(tx *libmodel.Tx) (err error) {
				ctx.log.V(3).Info(
					"DataCenter not found; event ignored.",
					"event",
					event)
				return
			}
			err = nil
		}
	}()
	switch event.code() {
	case USER_ADD_STORAGE_POOL:
		object := &DataCenter{}
		err = ctx.client.get(event.DataCenter.Ref, object)
		if err != nil {
			break
		}
		updater = func(tx *libmodel.Tx) (err error) {
			m := &model.DataCenter{
				Base: model.Base{ID: object.ID},
			}
			object.ApplyTo(m)
			err = tx.Insert(m)
			return
		}
	case USER_UPDATE_STORAGE_POOL:
		object := &DataCenter{}
		err = ctx.client.get(event.DataCenter.Ref, object)
		if err != nil {
			break
		}
		updater = func(tx *libmodel.Tx) (err error) {
			m := &model.DataCenter{
				Base: model.Base{ID: object.ID},
			}
			err = tx.Get(m)
			if err != nil {
				return
			}
			object.ApplyTo(m)
			err = tx.Update(m)
			return
		}
	case USER_REMOVE_STORAGE_POOL:
		updater = func(tx *libmodel.Tx) (err error) {
			err = tx.Delete(
				&model.DataCenter{
					Base: model.Base{ID: event.DataCenter.ID},
				})
			return
		}
	default:
		err = liberr.New("unknown event", "event", event)
	}

	return
}

// Network adapter.
type NetworkAdapter struct {
	BaseAdapter
}

// Handled events.
func (r *NetworkAdapter) Event() []int {
	return []int{
		NETWORK_ADD_NETWORK,
		NETWORK_UPDATE_NETWORK,
		NETWORK_REMOVE_NETWORK,
	}
}

// List the collection.
func (r *NetworkAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	networkList := NetworkList{}
	err = ctx.client.list("networks", &networkList, r.follow())
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range networkList.Items {
		m := &model.Network{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		list.Append(m)
	}

	itr = list.Iter()

	return
}

// Apply and event tot the inventory model.
func (r *NetworkAdapter) Apply(ctx *Context, event *Event) (updater Updater, err error) {
	var desired fb.Iterator
	desired, err = r.List(ctx)
	if err != nil {
		return
	}
	updater = func(tx *libmodel.Tx) (err error) {
		stored, err := tx.Find(
			&model.Network{},
			model.ListOptions{
				Detail: model.MaxDetail,
			})
		if err != nil {
			return
		}
		collection := libcnt.Collection{
			Stored: stored,
			Tx:     tx,
		}
		switch event.code() {
		case NETWORK_ADD_NETWORK:
			err = collection.Add(desired)
		case NETWORK_UPDATE_NETWORK:
			err = collection.Update(desired)
		case NETWORK_REMOVE_NETWORK:
			err = collection.Delete(desired)
		default:
			err = liberr.New("unknown event", "event", event)
		}
		return
	}

	return
}

func (r *NetworkAdapter) follow() libweb.Param {
	return r.BaseAdapter.follow(
		"vnic_profiles")
}

// NICProfileAdapter adapter.
type NICProfileAdapter struct {
	BaseAdapter
}

// Handled events.
func (r *NICProfileAdapter) Event() []int {
	return []int{
		// Profile.
		ADD_VNIC_PROFILE,
		UPDATE_VNIC_PROFILE,
		REMOVE_VNIC_PROFILE,
		// Network
		NETWORK_REMOVE_NETWORK,
	}
}

// List the collection.
func (r *NICProfileAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	pList := NICProfileList{}
	err = ctx.client.list("vnicprofiles", &pList)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range pList.Items {
		m := &model.NICProfile{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		list.Append(m)

	}

	itr = list.Iter()

	return
}

// Apply and event tot the inventory model.
func (r *NICProfileAdapter) Apply(ctx *Context, event *Event) (updater Updater, err error) {
	var desired fb.Iterator
	desired, err = r.List(ctx)
	if err != nil {
		return
	}
	updater = func(tx *libmodel.Tx) (err error) {
		stored, err := tx.Find(
			&model.NICProfile{},
			model.ListOptions{
				Detail: model.MaxDetail,
			})
		if err != nil {
			return
		}
		collection := libcnt.Collection{
			Stored: stored,
			Tx:     tx,
		}
		switch event.code() {
		case ADD_VNIC_PROFILE:
			err = collection.Add(desired)
		case UPDATE_VNIC_PROFILE:
			err = collection.Update(desired)
		case REMOVE_VNIC_PROFILE,
			NETWORK_REMOVE_NETWORK:
			err = collection.Delete(desired)
		default:
			err = liberr.New("unknown event", "event", event)
		}
		return
	}

	return
}

// DiskProfile adapter.
type DiskProfileAdapter struct {
	BaseAdapter
}

// Handled events.
func (r *DiskProfileAdapter) Event() []int {
	return []int{
		// Profile.
		USER_ADD_DISK_PROFILE,
		USER_UPDATE_DISK_PROFILE,
		USER_REMOVE_DISK_PROFILE,
		// StorageDomain
		USER_REMOVE_STORAGE_DOMAIN,
		USER_FORCE_REMOVE_STORAGE_DOMAIN,
	}
}

// List the collection.
func (r *DiskProfileAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	dList := DiskProfileList{}
	err = ctx.client.list("diskprofiles", &dList)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range dList.Items {
		m := &model.DiskProfile{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		list.Append(m)
	}

	itr = list.Iter()

	return
}

// Apply and event tot the inventory model.
func (r *DiskProfileAdapter) Apply(ctx *Context, event *Event) (updater Updater, err error) {
	var desired fb.Iterator
	desired, err = r.List(ctx)
	if err != nil {
		return
	}
	updater = func(tx *libmodel.Tx) (err error) {
		stored, err := tx.Find(
			&model.DiskProfile{},
			model.ListOptions{
				Detail: model.MaxDetail,
			})
		if err != nil {
			return
		}
		collection := libcnt.Collection{
			Stored: stored,
			Tx:     tx,
		}
		switch event.code() {
		case USER_ADD_DISK_PROFILE:
			err = collection.Add(desired)
		case USER_UPDATE_DISK_PROFILE:
			err = collection.Update(desired)
		case USER_REMOVE_DISK_PROFILE,
			USER_REMOVE_STORAGE_DOMAIN,
			USER_FORCE_REMOVE_STORAGE_DOMAIN:
			err = collection.Delete(desired)
		default:
			err = liberr.New("unknown event", "event", event)
		}
		return
	}

	return
}

// StorageDomain adapter.
type StorageDomainAdapter struct {
	BaseAdapter
}

// Handled events.
func (r *StorageDomainAdapter) Event() []int {
	return []int{
		USER_ADD_STORAGE_DOMAIN,
		USER_UPDATE_STORAGE_DOMAIN,
		USER_REMOVE_STORAGE_DOMAIN,
		USER_FORCE_REMOVE_STORAGE_DOMAIN,
		USER_DETACH_STORAGE_DOMAIN_FROM_POOL,
	}
}

// List the collection.
func (r *StorageDomainAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	sdList := StorageDomainList{}
	err = ctx.client.list("storagedomains", &sdList)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range sdList.Items {
		m := &model.StorageDomain{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		list.Append(m)
	}

	itr = list.Iter()

	return
}

// Apply and event tot the inventory model.
func (r *StorageDomainAdapter) Apply(ctx *Context, event *Event) (updater Updater, err error) {
	var desired fb.Iterator
	desired, err = r.List(ctx)
	if err != nil {
		return
	}
	updater = func(tx *libmodel.Tx) (err error) {
		stored, err := tx.Find(
			&model.StorageDomain{},
			model.ListOptions{
				Detail: model.MaxDetail,
			})
		if err != nil {
			return
		}
		collection := libcnt.Collection{
			Stored: stored,
			Tx:     tx,
		}
		switch event.code() {
		case USER_ADD_STORAGE_DOMAIN:
			err = collection.Add(desired)
		case USER_UPDATE_STORAGE_DOMAIN,
			USER_DETACH_STORAGE_DOMAIN_FROM_POOL:
			err = collection.Update(desired)
		case USER_REMOVE_STORAGE_DOMAIN,
			USER_FORCE_REMOVE_STORAGE_DOMAIN:
			err = collection.Delete(desired)
		default:
			err = liberr.New("unknown event", "event", event)
		}
		return
	}

	return
}

// Cluster adapter.
type ClusterAdapter struct {
	BaseAdapter
}

// Handled events.
func (r *ClusterAdapter) Event() []int {
	return []int{
		USER_ADD_CLUSTER,
		USER_UPDATE_CLUSTER,
		USER_REMOVE_CLUSTER,
	}
}

// List the collection.
func (r *ClusterAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	clusterList := ClusterList{}
	err = ctx.client.list("clusters", &clusterList)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range clusterList.Items {
		m := &model.Cluster{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		list.Append(m)
	}

	itr = list.Iter()

	return
}

// Apply and event tot the inventory model.
func (r *ClusterAdapter) Apply(ctx *Context, event *Event) (updater Updater, err error) {
	defer func() {
		if errors.Is(err, &NotFound{}) {
			updater = func(tx *libmodel.Tx) (err error) {
				ctx.log.V(3).Info(
					"Cluster not found; event ignored.",
					"event",
					event)
				return
			}
			err = nil
		}
	}()
	switch event.code() {
	case USER_ADD_CLUSTER:
		object := &Cluster{}
		err = ctx.client.get(event.Cluster.Ref, object)
		if err != nil {
			break
		}
		updater = func(tx *libmodel.Tx) (err error) {
			m := &model.Cluster{
				Base: model.Base{ID: object.ID},
			}
			object.ApplyTo(m)
			err = tx.Insert(m)
			return
		}
	case USER_UPDATE_CLUSTER:
		object := &Cluster{}
		err = ctx.client.get(event.Cluster.Ref, object)
		if err != nil {
			break
		}
		updater = func(tx *libmodel.Tx) (err error) {
			m := &model.Cluster{
				Base: model.Base{ID: object.ID},
			}
			err = tx.Get(m)
			if err != nil {
				return
			}
			object.ApplyTo(m)
			err = tx.Update(m)
			return
		}
	case USER_REMOVE_CLUSTER:
		updater = func(tx *libmodel.Tx) (err error) {
			err = tx.Delete(
				&model.Cluster{
					Base: model.Base{ID: event.Cluster.ID},
				})
			return
		}
	default:
		err = liberr.New("unknown event", "event", event)
	}

	return
}

// ServerCPUAdapter adapter.
type ServerCPUAdapter struct {
	BaseAdapter
}

// Handled events.
func (r *ServerCPUAdapter) Event() []int {
	return []int{}
}

// List the collection.
func (r *ServerCPUAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	serverCpuList := ServerCpu{}
	err = ctx.client.list("options/ServerCPUList", &serverCpuList)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range serverCpuList.Values.SystemOptionValues {
		m := &model.ServerCpu{
			Base: model.Base{ID: object.Version},
			SystemOptionValue: model.SystemOptionValue{
				Value:   object.Value,
				Version: object.Version,
			},
		}
		list.Append(m)
	}

	itr = list.Iter()

	return
}

// Apply and event tot the inventory model.
func (r *ServerCPUAdapter) Apply(ctx *Context, event *Event) (updater Updater, err error) {
	defer func() {
		if errors.Is(err, &NotFound{}) {
			updater = func(_ *libmodel.Tx) (err error) {
				ctx.log.V(3).Info(
					"ServerCPU not found; event ignored.",
					"event",
					event)
				return
			}
			err = nil
		}
	}()
	return
}

// Host (VDS) adapter.
type HostAdapter struct {
	BaseAdapter
}

// Handled events.
func (r *HostAdapter) Event() []int {
	return []int{
		USER_ADD_VDS,
		USER_UPDATE_VDS,
		USER_REMOVE_VDS,
		USER_VDS_MAINTENANCE,
		USER_VDS_MAINTENANCE_WITHOUT_REASON,
		USER_VDS_MAINTENANCE_MANUAL_HA,
		VDS_DETECTED,
	}
}

// List the collection.
func (r *HostAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	hostList := HostList{}
	err = ctx.client.list("hosts", &hostList, r.follow())
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range hostList.Items {
		m := &model.Host{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		list.Append(m)
	}

	itr = list.Iter()

	return
}

// Apply and event tot the inventory model.
func (r *HostAdapter) Apply(ctx *Context, event *Event) (updater Updater, err error) {
	defer func() {
		if errors.Is(err, &NotFound{}) {
			updater = func(tx *libmodel.Tx) (err error) {
				ctx.log.V(3).Info(
					"Host not found; event ignored.",
					"event",
					event)
				return
			}
			err = nil
		}
	}()
	switch event.code() {
	case USER_ADD_VDS:
		object := &Host{}
		err = ctx.client.get(event.Host.Ref, object, r.follow())
		if err != nil {
			break
		}
		updater = func(tx *libmodel.Tx) (err error) {
			m := &model.Host{
				Base: model.Base{ID: object.ID},
			}
			object.ApplyTo(m)
			err = tx.Insert(m)
			return
		}
	case USER_UPDATE_VDS,
		USER_VDS_MAINTENANCE,
		USER_VDS_MAINTENANCE_WITHOUT_REASON,
		USER_VDS_MAINTENANCE_MANUAL_HA,
		VDS_DETECTED:
		object := &Host{}
		err = ctx.client.get(event.Host.Ref, object, r.follow())
		if err != nil {
			break
		}
		updater = func(tx *libmodel.Tx) (err error) {
			m := &model.Host{
				Base: model.Base{ID: object.ID},
			}
			err = tx.Get(m)
			if err != nil {
				return
			}
			object.ApplyTo(m)
			err = tx.Update(m)
			return
		}
	case USER_REMOVE_VDS:
		updater = func(tx *libmodel.Tx) (err error) {
			err = tx.Delete(
				&model.Host{
					Base: model.Base{ID: event.Host.ID},
				})
			return
		}
	default:
		err = liberr.New("unknown event", "event", event)
	}

	return
}

func (r *HostAdapter) follow() libweb.Param {
	return r.BaseAdapter.follow(
		"network_attachments",
		"nics",
	)
}

// VM adapter.
type VMAdapter struct {
	BaseAdapter
}

// Handled events.
func (r *VMAdapter) Event() []int {
	return []int{
		// Add
		USER_ADD_VM,
		USER_ADD_VM_FINISHED_SUCCESS,
		IMPORTEXPORT_IMPORT_VM,
		// Update
		USER_UPDATE_VM,
		SYSTEM_UPDATE_VM,
		USER_UPDATE_VM_DISK,
		USER_ADD_DISK_TO_VM_SUCCESS,
		USER_REMOVE_DISK_FROM_VM,
		USER_FINISHED_REMOVE_DISK_ATTACHED_TO_VMS,
		USER_ATTACH_DISK_TO_VM,
		USER_DETACH_DISK_FROM_VM,
		USER_EJECT_VM_DISK,
		NETWORK_USER_ADD_VM_INTERFACE,
		NETWORK_USER_UPDATE_VM_INTERFACE,
		NETWORK_USER_REMOVE_VM_INTERFACE,
		USER_CREATE_SNAPSHOT_FINISHED_SUCCESS,
		USER_REMOVE_SNAPSHOT_FINISHED_SUCCESS,
		VM_ADD_HOST_DEVICES,
		VM_REMOVE_HOST_DEVICES,
		USER_RUN_VM,
		USER_PAUSE_VM,
		USER_RESUME_VM,
		USER_SUSPEND_VM_OK,
		VM_DOWN,
		// Delete
		USER_REMOVE_VM_FINISHED_INTERNAL,
		USER_REMOVE_VM_FINISHED_ILLEGAL_DISKS,
		USER_REMOVE_VM_FINISHED_ILLEGAL_DISKS_INTERNAL,
		USER_REMOVE_VM,
		// StorageDomain.
		USER_DETACH_STORAGE_DOMAIN_FROM_POOL,
		USER_FORCE_REMOVE_STORAGE_DOMAIN,
	}
}

// List the collection.
func (r *VMAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	vmList := VMList{}
	err = ctx.client.list("vms", &vmList, r.follow())
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range vmList.Items {
		m := &model.VM{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		list.Append(m)
	}

	itr = list.Iter()
	return
}

// Apply and event tot the inventory model.
func (r *VMAdapter) Apply(ctx *Context, event *Event) (updater Updater, err error) {
	defer func() {
		if errors.Is(err, &NotFound{}) {
			updater = func(tx *libmodel.Tx) (err error) {
				ctx.log.V(3).Info(
					"VM not found; event ignored.",
					"event",
					event)
				return
			}
			err = nil
		}
	}()
	switch event.code() {
	case USER_ADD_VM,
		USER_ADD_VM_FINISHED_SUCCESS,
		IMPORTEXPORT_IMPORT_VM:
		object := &VM{}
		err = ctx.client.get(event.VM.Ref, object, r.follow())
		if err != nil {
			return
		}
		updater = func(tx *libmodel.Tx) (err error) {
			m := &model.VM{
				Base: model.Base{ID: object.ID},
			}
			object.ApplyTo(m)
			err = tx.Insert(m)
			return
		}
	case USER_UPDATE_VM,
		SYSTEM_UPDATE_VM,
		USER_UPDATE_VM_DISK,
		USER_ADD_DISK_TO_VM_SUCCESS,
		USER_REMOVE_DISK_FROM_VM,
		USER_ATTACH_DISK_TO_VM,
		USER_DETACH_DISK_FROM_VM,
		USER_EJECT_VM_DISK,
		USER_CHANGE_DISK_VM,
		NETWORK_USER_ADD_VM_INTERFACE,
		NETWORK_USER_UPDATE_VM_INTERFACE,
		NETWORK_USER_REMOVE_VM_INTERFACE,
		USER_CREATE_SNAPSHOT_FINISHED_SUCCESS,
		USER_REMOVE_SNAPSHOT_FINISHED_SUCCESS,
		VM_ADD_HOST_DEVICES,
		VM_REMOVE_HOST_DEVICES,
		USER_RUN_VM,
		USER_PAUSE_VM,
		USER_RESUME_VM,
		USER_SUSPEND_VM_OK,
		VM_DOWN:
		object := &VM{}
		err = ctx.client.get(event.VM.Ref, object, r.follow())
		if err != nil {
			break
		}
		updater = func(tx *libmodel.Tx) (err error) {
			m := &model.VM{
				Base: model.Base{ID: object.ID},
			}
			err = tx.Get(m)
			if err != nil {
				return
			}
			object.ApplyTo(m)
			err = tx.Update(m)
			return
		}
	case USER_REMOVE_VM_FINISHED_INTERNAL,
		USER_REMOVE_VM_FINISHED_ILLEGAL_DISKS,
		USER_REMOVE_VM_FINISHED_ILLEGAL_DISKS_INTERNAL,
		USER_REMOVE_VM:
		updater = func(tx *libmodel.Tx) (err error) {
			err = tx.Delete(
				&model.VM{
					Base: model.Base{ID: event.VM.ID},
				})
			return
		}
	case USER_FINISHED_REMOVE_DISK_ATTACHED_TO_VMS,
		USER_DETACH_STORAGE_DOMAIN_FROM_POOL,
		USER_FORCE_REMOVE_STORAGE_DOMAIN:
		var desired fb.Iterator
		desired, err = r.List(ctx)
		if err != nil {
			return
		}
		updater = func(tx *libmodel.Tx) (err error) {
			stored, err := tx.Find(
				&model.VM{},
				model.ListOptions{
					Detail: model.MaxDetail,
				})
			if err != nil {
				return
			}
			collection := libcnt.Collection{
				Stored: stored,
				Tx:     tx,
			}
			err = collection.Reconcile(desired)
			return
		}
	default:
		err = liberr.New("unknown event", "event", event)
	}

	return
}

func (r *VMAdapter) follow() libweb.Param {
	return r.BaseAdapter.follow(
		"disk_attachments",
		"host_devices",
		"snapshots",
		"watchdogs",
		"cdroms",
		"nics",
	)
}

// Disk adapter.
type DiskAdapter struct {
	BaseAdapter
}

// Handled events.
func (r *DiskAdapter) Event() []int {
	return []int{
		// Disk
		USER_ADD_DISK_FINISHED_SUCCESS,
		USER_ADD_DISK_TO_VM_SUCCESS,
		USER_REMOVE_DISK,
		USER_UPDATE_VM_DISK,
		// VM
		USER_FINISHED_REMOVE_DISK_ATTACHED_TO_VMS,
		USER_REMOVE_DISK_FROM_VM,
		USER_FORCE_REMOVE_STORAGE_DOMAIN,
		USER_ADD_VM,
		USER_ADD_VM_FINISHED_SUCCESS,
		IMPORTEXPORT_IMPORT_VM,
		USER_REMOVE_VM,
		// StorageDomain.
		USER_DETACH_STORAGE_DOMAIN_FROM_POOL,
		USER_FORCE_REMOVE_STORAGE_DOMAIN,
	}
}

// List the collection.
func (r *DiskAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	diskList := DiskList{}
	err = ctx.client.list("disks", &diskList)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range diskList.Items {
		if object.StorageType == "lun" {
			err = ctx.client.list(fmt.Sprintf("disks/%s", object.ID), &object)
			if err != nil {
				return
			}
		}
		m := &model.Disk{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		list.Append(m)
	}

	itr = list.Iter()

	return
}

// Apply and event tot the inventory model.
// Disks may be added and deleted when VMs are created
// and deleted without generating any disk events.
func (r *DiskAdapter) Apply(ctx *Context, event *Event) (updater Updater, err error) {
	var desired fb.Iterator
	desired, err = r.List(ctx)
	if err != nil {
		return
	}
	updater = func(tx *libmodel.Tx) (err error) {
		stored, err := tx.Find(
			&model.Disk{},
			model.ListOptions{
				Detail: model.MaxDetail,
			})
		if err != nil {
			return
		}
		collection := libcnt.Collection{
			Stored: stored,
			Tx:     tx,
		}
		switch event.code() {
		case USER_ADD_DISK_FINISHED_SUCCESS,
			USER_ADD_DISK_TO_VM_SUCCESS:
			err = collection.Add(desired)
		case USER_REMOVE_DISK,
			USER_REMOVE_DISK_FROM_VM,
			USER_FINISHED_REMOVE_DISK_ATTACHED_TO_VMS,
			USER_DETACH_STORAGE_DOMAIN_FROM_POOL,
			USER_FORCE_REMOVE_STORAGE_DOMAIN:
			err = collection.Delete(desired)
		case USER_ADD_VM,
			USER_ADD_VM_FINISHED_SUCCESS,
			IMPORTEXPORT_IMPORT_VM,
			USER_UPDATE_VM_DISK,
			USER_REMOVE_VM:
			err = collection.Reconcile(desired)
		default:
			err = liberr.New("unknown event", "event", event)
		}

		return
	}

	return
}
