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
	// Cluster
	USER_ADD_CLUSTER    = 809
	USER_UPDATE_CLUSTER = 811
	USER_REMOVE_CLUSTER = 813
	// Host
	USER_ADD_HOST    = 42
	USER_UPDATE_HOST = 43
	USER_REMOVE_HOST = 44
	// VM
	USER_ADD_VM    = 34
	USER_UPDATE_VM = 35
	USER_REMOVE_VM = 113
)

//
// All adapters.
var adapterList []Adapter

//
// Event (type) mapped to adapter.
var adapterMap = map[int]Adapter{}

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
			adapterMap[event] = adapter
		}
	}
}

//
// Model adapter.
// Provides integration between the REST resource
// model and the inventory model.
type Adapter interface {
	// List REST collections.
	List(client *Client) (itr fb.Iterator, err error)
	// Apply and event to the inventory model.
	Apply(client *Client, tx *libmodel.Tx, event *Event) (err error)
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
func (r *DataCenterAdapter) Apply(client *Client, tx *libmodel.Tx, event *Event) (err error) {
	switch event.code() {
	case USER_ADD_STORAGE_POOL:
		object := &DataCenter{}
		err = client.get(event.DataCenter.Ref, object)
		if err != nil {
			break
		}
		m := &model.DataCenter{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		err = tx.Insert(m)
		if err != nil {
			break
		}
	case USER_UPDATE_STORAGE_POOL:
		object := &DataCenter{}
		err = client.get(event.DataCenter.Ref, object)
		if err != nil {
			break
		}
		m := &model.DataCenter{
			Base: model.Base{ID: object.ID},
		}
		err = tx.Get(m)
		if err != nil {
			break
		}
		object.ApplyTo(m)
		err = tx.Update(m)
		if err != nil {
			break
		}
	case USER_REMOVE_STORAGE_POOL:
		m := &model.Cluster{
			Base: model.Base{ID: event.DataCenter.Ref},
		}
		err = tx.Delete(m)
		if err != nil {
			break
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
func (r *NetworkAdapter) Apply(client *Client, tx *libmodel.Tx, event *Event) (err error) {
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
func (r *NICProfileAdapter) Apply(client *Client, tx *libmodel.Tx, event *Event) (err error) {
	desired, err := r.List(client)
	if err != nil {
		return
	}
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

//
// DiskProfile adapter.
type DiskProfileAdapter struct {
}

//
// Handled events.
func (r *DiskProfileAdapter) Event() []int {
	return []int{}
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
func (r *DiskProfileAdapter) Apply(client *Client, tx *libmodel.Tx, event *Event) (err error) {
	switch event.code() {
	default:
		err = liberr.New("unknown event", "event", event)
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
func (r *StorageDomainAdapter) Apply(client *Client, tx *libmodel.Tx, event *Event) (err error) {
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
func (r *ClusterAdapter) Apply(client *Client, tx *libmodel.Tx, event *Event) (err error) {
	switch event.code() {
	case USER_ADD_CLUSTER:
		object := &Cluster{}
		err = client.get(event.Cluster.Ref, object)
		if err != nil {
			break
		}
		m := &model.Cluster{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		err = tx.Insert(m)
		if err != nil {
			break
		}
	case USER_UPDATE_CLUSTER:
		object := &Cluster{}
		err = client.get(event.Cluster.Ref, object)
		if err != nil {
			break
		}
		m := &model.Cluster{
			Base: model.Base{ID: object.ID},
		}
		err = tx.Get(m)
		if err != nil {
			break
		}
		object.ApplyTo(m)
		err = tx.Update(m)
		if err != nil {
			break
		}
	case USER_REMOVE_CLUSTER:
		m := &model.Cluster{
			Base: model.Base{ID: event.Cluster.Ref},
		}
		err = tx.Delete(m)
		if err != nil {
			break
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
func (r *HostAdapter) Apply(client *Client, tx *libmodel.Tx, event *Event) (err error) {
	switch event.code() {
	case USER_ADD_HOST:
		object := &Host{}
		err = client.get(event.Host.Ref, object, r.follow())
		if err != nil {
			break
		}
		m := &model.Host{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		err = tx.Insert(m)
		if err != nil {
			break
		}
	case USER_UPDATE_HOST:
		object := &Host{}
		err = client.get(event.Host.Ref, object, r.follow())
		if err != nil {
			break
		}
		m := &model.Host{
			Base: model.Base{ID: object.ID},
		}
		err = tx.Get(m)
		if err != nil {
			break
		}
		object.ApplyTo(m)
		err = tx.Update(m)
		if err != nil {
			break
		}
	case USER_REMOVE_HOST:
		m := &model.Host{
			Base: model.Base{ID: event.Host.Ref},
		}
		err = tx.Delete(m)
		if err != nil {
			break
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
		USER_ADD_VM,
		USER_UPDATE_VM,
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
func (r *VMAdapter) Apply(client *Client, tx *libmodel.Tx, event *Event) (err error) {
	switch event.code() {
	case USER_ADD_VM:
		object := &VM{}
		err = client.get(event.VM.Ref, object, r.follow())
		if err != nil {
			break
		}
		m := &model.VM{
			Base: model.Base{ID: object.ID},
		}
		object.ApplyTo(m)
		err = tx.Insert(m)
		if err != nil {
			break
		}
	case USER_UPDATE_VM:
		object := &VM{}
		err = client.get(event.VM.Ref, object, r.follow())
		if err != nil {
			break
		}
		m := &model.VM{
			Base: model.Base{ID: object.ID},
		}
		err = tx.Get(m)
		if err != nil {
			break
		}
		object.ApplyTo(m)
		err = tx.Update(m)
		if err != nil {
			break
		}
	case USER_REMOVE_VM:
		m := &model.VM{
			Base: model.Base{ID: event.VM.Ref},
		}
		err = tx.Delete(m)
		if err != nil {
			break
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
	return []int{}
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
func (r *DiskAdapter) Apply(client *Client, tx *libmodel.Tx, event *Event) (err error) {
	switch event.code() {
	default:
		err = liberr.New("unknown event", "event", event)
	}

	return
}
