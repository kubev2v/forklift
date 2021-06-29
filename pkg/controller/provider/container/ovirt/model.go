package ovirt

import (
	liberr "github.com/konveyor/controller/pkg/error"
	fb "github.com/konveyor/controller/pkg/filebacked"
	libcnt "github.com/konveyor/controller/pkg/inventory/container"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ovirt"
	"strings"
)

//
// Event codes.
const (
	// DataCenter
	USER_ADD_STORAGE_POOL    = 950
	USER_UPDATE_STORAGE_POOL = 952
	USER_REMOVE_STORAGE_POOL = 954
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
	USER_ADD_HOST    = 42
	USER_UPDATE_HOST = 43
	USER_REMOVE_HOST = 44
	// VM
	USER_ADD_VM                           = 34
	USER_UPDATE_VM                        = 35
	USER_REMOVE_VM                        = 113
	USER_ADD_DISK_TO_VM_SUCCESS           = 97
	USER_UPDATE_VM_DISK                   = 88
	USER_REMOVE_DISK_FROM_VM              = 80
	USER_ATTACH_DISK_TO_VM                = 2016
	USER_DETACH_DISK_FROM_VM              = 2018
	USER_EJECT_VM_DISK                    = 521
	NETWORK_USER_ADD_VM_INTERFACE         = 932
	NETWORK_USER_UPDATE_VM_INTERFACE      = 934
	NETWORK_USER_REMOVE_VM_INTERFACE      = 930
	USER_CREATE_SNAPSHOT_FINISHED_SUCCESS = 68
	USER_REMOVE_SNAPSHOT_FINISHED_SUCCESS = 356
	USER_RUN_VM                           = 32
	USER_SUSPEND_VM_OK                    = 503
	USER_PAUSE_VM                         = 39
	USER_RESUME_VM                        = 40
	VM_DOWN                               = 61
	// Disk
	USER_ADD_DISK_FINISHED_SUCCESS            = 2021
	USER_REMOVE_DISK                          = 2014
	USER_FINISHED_REMOVE_DISK_ATTACHED_TO_VMS = 2042
)

//
// All adapters.
var adapterList []Adapter

//
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

//
// Updates the DB based on
// changes described by an Event.
type Updater func(tx *libmodel.Tx) error

//
// Model adapter.
// Provides integration between the REST resource
// model and the inventory model.
type Adapter interface {
	// List REST collections.
	List(client *Client) (itr fb.Iterator, err error)
	// Apply an event to the inventory model.
	Apply(event *Event, client *Client) (updater Updater, err error)
	// List handled event (codes).
	Event() []int
}

//
// DataCenter.
type DataCenterAdapter struct {
}

//
// List the collection.
func (r *DataCenterAdapter) List(client *Client) (itr fb.Iterator, err error) {
	dataCenterList := DataCenterList{}
	err = client.list("datacenters", &dataCenterList)
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

//
// Handled events.
func (r *DataCenterAdapter) Event() []int {
	return []int{
		USER_ADD_STORAGE_POOL,
		USER_UPDATE_STORAGE_POOL,
		USER_REMOVE_STORAGE_POOL,
	}
}

//
// Apply events to the inventory model.
func (r *DataCenterAdapter) Apply(event *Event, client *Client) (updater Updater, err error) {
	switch event.code() {
	case USER_ADD_STORAGE_POOL:
		object := &DataCenter{}
		err = client.get(event.DataCenter.Ref, object)
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
		err = client.get(event.DataCenter.Ref, object)
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
				&model.Cluster{
					Base: model.Base{ID: event.DataCenter.Ref},
				})
			return
		}
	default:
		err = liberr.New("unknown event", "event", event)
	}

	return
}

//
// Network adapter.
type NetworkAdapter struct {
}

//
// Handled events.
func (r *NetworkAdapter) Event() []int {
	return []int{}
}

//
// List the collection.
func (r *NetworkAdapter) List(client *Client) (itr fb.Iterator, err error) {
	networkList := NetworkList{}
	err = client.list("networks", &networkList, r.follow())
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

//
// Apply and event tot the inventory model.
func (r *NetworkAdapter) Apply(event *Event, client *Client) (updater Updater, err error) {
	switch event.code() {
	default:
		err = liberr.New("unknown event", "event", event)
	}

	return
}

func (r *NetworkAdapter) follow() libweb.Param {
	return libweb.Param{
		Key: "follow",
		Value: strings.Join(
			[]string{
				"vnic_profiles",
			},
			","),
	}
}

//
// NICProfileAdapter adapter.
type NICProfileAdapter struct {
}

//
// Handled events.
func (r *NICProfileAdapter) Event() []int {
	return []int{
		ADD_VNIC_PROFILE,
		UPDATE_VNIC_PROFILE,
		REMOVE_VNIC_PROFILE,
	}
}

//
// List the collection.
func (r *NICProfileAdapter) List(client *Client) (itr fb.Iterator, err error) {
	pList := NICProfileList{}
	err = client.list("vnicprofiles", &pList)
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

//
// Apply and event tot the inventory model.
func (r *NICProfileAdapter) Apply(event *Event, client *Client) (updater Updater, err error) {
	var desired fb.Iterator
	desired, err = r.List(client)
	if err != nil {
		return
	}
	updater = func(tx *libmodel.Tx) (err error) {
		stored, err := tx.Iter(
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
		case REMOVE_VNIC_PROFILE:
			err = collection.Delete(desired)
		default:
			err = liberr.New("unknown event", "event", event)
		}
		return
	}

	return
}

//
// DiskProfile adapter.
type DiskProfileAdapter struct {
}

//
// Handled events.
func (r *DiskProfileAdapter) Event() []int {
	return []int{
		USER_ADD_DISK_PROFILE,
		USER_UPDATE_DISK_PROFILE,
		USER_REMOVE_DISK_PROFILE,
	}
}

//
// List the collection.
func (r *DiskProfileAdapter) List(client *Client) (itr fb.Iterator, err error) {
	dList := DiskProfileList{}
	err = client.list("diskprofiles", &dList)
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

//
// Apply and event tot the inventory model.
func (r *DiskProfileAdapter) Apply(event *Event, client *Client) (updater Updater, err error) {
	var desired fb.Iterator
	desired, err = r.List(client)
	if err != nil {
		return
	}
	updater = func(tx *libmodel.Tx) (err error) {
		stored, err := tx.Iter(
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
		case USER_REMOVE_DISK_PROFILE:
			err = collection.Delete(desired)
		default:
			err = liberr.New("unknown event", "event", event)
		}
		return
	}

	return
}

//
// StorageDomain adapter.
type StorageDomainAdapter struct {
}

//
// Handled events.
func (r *StorageDomainAdapter) Event() []int {
	return []int{}
}

//
// List the collection.
func (r *StorageDomainAdapter) List(client *Client) (itr fb.Iterator, err error) {
	sdList := StorageDomainList{}
	err = client.list("storagedomains", &sdList)
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

//
// Apply and event tot the inventory model.
func (r *StorageDomainAdapter) Apply(event *Event, client *Client) (updater Updater, err error) {
	switch event.code() {
	default:
		err = liberr.New("unknown event", "event", event)
	}

	return
}

//
// Cluster adapter.
type ClusterAdapter struct {
}

//
// Handled events.
func (r *ClusterAdapter) Event() []int {
	return []int{
		USER_ADD_CLUSTER,
		USER_UPDATE_CLUSTER,
		USER_REMOVE_CLUSTER,
	}
}

//
// List the collection.
func (r *ClusterAdapter) List(client *Client) (itr fb.Iterator, err error) {
	clusterList := ClusterList{}
	err = client.list("clusters", &clusterList)
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

//
// Apply and event tot the inventory model.
func (r *ClusterAdapter) Apply(event *Event, client *Client) (updater Updater, err error) {
	switch event.code() {
	case USER_ADD_CLUSTER:
		object := &Cluster{}
		err = client.get(event.Cluster.Ref, object)
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
		err = client.get(event.Cluster.Ref, object)
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
					Base: model.Base{ID: event.Cluster.Ref},
				})
			return
		}
	default:
		err = liberr.New("unknown event", "event", event)
	}

	return
}

//
// Host adapter.
type HostAdapter struct {
}

//
// Handled events.
func (r *HostAdapter) Event() []int {
	return []int{
		USER_ADD_HOST,
		USER_UPDATE_HOST,
		USER_REMOVE_HOST,
	}
}

//
// List the collection.
func (r *HostAdapter) List(client *Client) (itr fb.Iterator, err error) {
	hostList := HostList{}
	err = client.list("hosts", &hostList, r.follow())
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

//
// Apply and event tot the inventory model.
func (r *HostAdapter) Apply(event *Event, client *Client) (updater Updater, err error) {
	switch event.code() {
	case USER_ADD_HOST:
		object := &Host{}
		err = client.get(event.Host.Ref, object, r.follow())
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
	case USER_UPDATE_HOST:
		object := &Host{}
		err = client.get(event.Host.Ref, object, r.follow())
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
	case USER_REMOVE_HOST:
		updater = func(tx *libmodel.Tx) (err error) {
			err = tx.Delete(
				&model.Host{
					Base: model.Base{ID: event.Host.Ref},
				})
			return
		}
	default:
		err = liberr.New("unknown event", "event", event)
	}

	return
}

func (r *HostAdapter) follow() libweb.Param {
	return libweb.Param{
		Key: "follow",
		Value: strings.Join(
			[]string{
				"network_attachments",
				"nics",
			},
			","),
	}
}

//
// VM adapter.
type VMAdapter struct {
}

//
// Handled events.
func (r *VMAdapter) Event() []int {
	return []int{
		// Add
		USER_ADD_VM,
		// Update
		USER_UPDATE_VM,
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
		USER_RUN_VM,
		USER_PAUSE_VM,
		USER_RESUME_VM,
		USER_SUSPEND_VM_OK,
		VM_DOWN,
		// Delete
		USER_REMOVE_VM,
	}
}

//
// List the collection.
func (r *VMAdapter) List(client *Client) (itr fb.Iterator, err error) {
	vmList := VMList{}
	err = client.list("vms", &vmList, r.follow())
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

//
// Apply and event tot the inventory model.
func (r *VMAdapter) Apply(event *Event, client *Client) (updater Updater, err error) {
	switch event.code() {
	case USER_ADD_VM:
		object := &VM{}
		err = client.get(event.VM.Ref, object, r.follow())
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
		USER_UPDATE_VM_DISK,
		USER_ADD_DISK_TO_VM_SUCCESS,
		USER_REMOVE_DISK_FROM_VM,
		USER_ATTACH_DISK_TO_VM,
		USER_DETACH_DISK_FROM_VM,
		USER_EJECT_VM_DISK,
		NETWORK_USER_ADD_VM_INTERFACE,
		NETWORK_USER_UPDATE_VM_INTERFACE,
		NETWORK_USER_REMOVE_VM_INTERFACE,
		USER_CREATE_SNAPSHOT_FINISHED_SUCCESS,
		USER_REMOVE_SNAPSHOT_FINISHED_SUCCESS,
		USER_RUN_VM,
		USER_PAUSE_VM,
		USER_RESUME_VM,
		USER_SUSPEND_VM_OK,
		VM_DOWN:
		object := &VM{}
		err = client.get(event.VM.Ref, object, r.follow())
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
	case USER_REMOVE_VM:
		updater = func(tx *libmodel.Tx) (err error) {
			err = tx.Delete(
				&model.VM{
					Base: model.Base{ID: event.VM.Ref},
				})
			return
		}
	case USER_FINISHED_REMOVE_DISK_ATTACHED_TO_VMS:
		var desired fb.Iterator
		desired, err = r.List(client)
		if err != nil {
			return
		}
		updater = func(tx *libmodel.Tx) (err error) {
			stored, err := tx.Iter(
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
			err = collection.Update(desired)
			return
		}
	default:
		err = liberr.New("unknown event", "event", event)
	}

	return
}

func (r *VMAdapter) follow() libweb.Param {
	return libweb.Param{
		Key: "follow",
		Value: strings.Join(
			[]string{
				"disk_attachments",
				"host_devices",
				"snapshots",
				"watchdogs",
				"cdroms",
				"nics",
			},
			","),
	}
}

//
// Disk adapter.
type DiskAdapter struct {
}

//
// Handled events.
func (r *DiskAdapter) Event() []int {
	return []int{
		USER_ADD_DISK_FINISHED_SUCCESS,
		USER_ADD_DISK_TO_VM_SUCCESS,
		USER_REMOVE_DISK,
		USER_REMOVE_DISK_FROM_VM,
		USER_FINISHED_REMOVE_DISK_ATTACHED_TO_VMS,
	}
}

//
// List the collection.
func (r *DiskAdapter) List(client *Client) (itr fb.Iterator, err error) {
	diskList := DiskList{}
	err = client.list("disks", &diskList)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range diskList.Items {
		m := &model.Disk{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		list.Append(m)
	}

	itr = list.Iter()

	return
}

//
// Apply and event tot the inventory model.
func (r *DiskAdapter) Apply(event *Event, client *Client) (updater Updater, err error) {
	var desired fb.Iterator
	desired, err = r.List(client)
	if err != nil {
		return
	}
	updater = func(tx *libmodel.Tx) (err error) {
		stored, err := tx.Iter(
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
			USER_FINISHED_REMOVE_DISK_ATTACHED_TO_VMS:
			err = collection.Delete(desired)
		default:
			err = liberr.New("unknown event", "event", event)
		}

		return
	}

	return
}
