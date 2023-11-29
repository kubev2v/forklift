package ova

import (
	"context"
	"errors"
	"fmt"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ova"
	fb "github.com/konveyor/forklift-controller/pkg/lib/filebacked"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
)

// All adapters.
var adapterList []Adapter

// Event (type) mapped to adapter.
var adapterMap = map[int][]Adapter{}

func init() {
	adapterList = []Adapter{
		&NetworkAdapter{},
		&DiskAdapter{},
		&VMAdapter{},
		&StorageAdapter{},
	}
}

// Updates the DB based on
// changes described by an Event.
type Updater func(tx *libmodel.Tx) error

// Adapter context.
type Context struct {
	// Context.
	ctx context.Context
	// OVA client.
	client *Client
	// Log.
	log logging.LevelLogger
	// DB client.
	db libmodel.DB
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
	List(ctx *Context, provider *api.Provider) (itr fb.Iterator, err error)
	// Get object updates
	GetUpdates(ctx *Context) (updater []Updater, err error)
	// Clean unexisting objects within the database
	DeleteUnexisting(ctx *Context) (deletions []Updater, err error)
}

// Base adapter.
type BaseAdapter struct {
}

// Network adapter.
type NetworkAdapter struct {
	BaseAdapter
}

// List the collection.
func (r *NetworkAdapter) List(ctx *Context, provider *api.Provider) (itr fb.Iterator, err error) {
	networkList := []Network{}
	err = ctx.client.list("networks", &networkList)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range networkList {
		m := &model.Network{
			Base: model.Base{Name: object.Name},
		}
		object.ApplyTo(m)
		list.Append(m)
	}

	itr = list.Iter()

	return
}

func (r *NetworkAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	networkList := []Network{}
	err = ctx.client.list("networks", &networkList)
	if err != nil {
		return
	}
	for i := range networkList {
		network := &networkList[i]
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Network{
				Base: model.Base{Name: network.Name},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, libmodel.NotFound) {
					network.ApplyTo(m)
					err = tx.Insert(m)
				}
				return
			}
			network.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	return
}

func (r *NetworkAdapter) DeleteUnexisting(ctx *Context) (deletions []Updater, err error) {
	networkList := []model.Network{}
	err = ctx.db.List(&networkList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	networkListServer := []Network{}
	err = ctx.client.list("networks", &networkListServer)
	if err != nil {
		return
	}

	elementMap := make(map[string]bool)
	for _, network := range networkListServer {
		elementMap[network.ID] = true
	}

	networksToDelete := []string{}
	for _, network := range networkList {
		if _, found := elementMap[network.ID]; !found {
			networksToDelete = append(networksToDelete, network.ID)
		}
	}
	for _, networkId := range networksToDelete {
		currentID := networkId
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Network{
				Base: model.Base{ID: currentID},
			}
			return tx.Delete(m)
		}
		deletions = append(deletions, updater)
	}
	return
}

// VM adapter.
type VMAdapter struct {
	BaseAdapter
}

// List the collection.
func (r *VMAdapter) List(ctx *Context, provider *api.Provider) (itr fb.Iterator, err error) {
	vmList := []VM{}
	err = ctx.client.list("vms", &vmList)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range vmList {
		m := &model.VM{
			Base: model.Base{ID: object.UUID},
		}
		object.ApplyTo(m)
		list.Append(m)
	}

	itr = list.Iter()
	return
}

// Get updates since last sync.
func (r *VMAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	vmList := []VM{}
	err = ctx.client.list("vms", &vmList)
	if err != nil {
		return
	}
	for i := range vmList {
		vm := &vmList[i]
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.VM{
				Base: model.Base{ID: vm.UUID},
			}
			if err = tx.Get(m); err != nil {
				if errors.Is(err, libmodel.NotFound) {
					vm.ApplyTo(m)
					err = tx.Insert(m)
				}
			} else if vm.OvaPath != m.OvaPath {
				vm.ApplyTo(m)
				err = tx.Update(m)
			}
			return
		}
		updates = append(updates, updater)
	}
	return
}

func (r *VMAdapter) DeleteUnexisting(ctx *Context) (deletions []Updater, err error) {
	vmList := []model.VM{}
	err = ctx.db.List(&vmList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	vmListServer := []VM{}
	err = ctx.client.list("vms", &vmListServer)
	if err != nil {
		return
	}

	elementMap := make(map[string]bool)
	for _, vm := range vmListServer {
		elementMap[vm.UUID] = true
	}

	vmsToDelete := []string{}
	for _, vm := range vmList {
		if _, found := elementMap[vm.ID]; !found {
			vmsToDelete = append(vmsToDelete, vm.ID)
		}
	}
	for _, vmId := range vmsToDelete {
		currentID := vmId
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.VM{
				Base: model.Base{ID: currentID},
			}
			return tx.Delete(m)
		}
		deletions = append(deletions, updater)
	}
	return
}

// Disk adapter.
type DiskAdapter struct {
	BaseAdapter
}

// List the collection.
func (r *DiskAdapter) List(ctx *Context, provider *api.Provider) (itr fb.Iterator, err error) {
	diskList := []Disk{}
	err = ctx.client.list("disks", &diskList)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, object := range diskList {
		m := &model.Disk{
			Base: model.Base{ID: object.DiskId},
		}
		object.ApplyTo(m)
		list.Append(m)
	}

	itr = list.Iter()

	return
}

func (r *DiskAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	diskList := []Disk{}
	err = ctx.client.list("disks", &diskList)
	if err != nil {
		return
	}
	for i := range diskList {
		disk := &diskList[i]
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Disk{
				Base: model.Base{ID: disk.DiskId},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, libmodel.NotFound) {
					disk.ApplyTo(m)
					err = tx.Insert(m)
				}
				return
			}
			disk.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	return
}

func (r *DiskAdapter) DeleteUnexisting(ctx *Context) (deletions []Updater, err error) {
	diskList := []model.Disk{}
	err = ctx.db.List(&diskList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	diskListServer := []Disk{}
	err = ctx.client.list("disks", &diskListServer)
	if err != nil {
		return
	}

	elementMap := make(map[string]bool)
	for _, disk := range diskListServer {
		elementMap[disk.ID] = true
	}

	disksToDelete := []string{}
	for _, disk := range diskList {
		if _, found := elementMap[disk.ID]; !found {
			disksToDelete = append(disksToDelete, disk.ID)
		}
	}
	for _, diskId := range disksToDelete {
		currentID := diskId
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Disk{
				Base: model.Base{ID: currentID},
			}
			return tx.Delete(m)
		}
		deletions = append(deletions, updater)
	}
	return
}

type StorageAdapter struct {
	BaseAdapter
}

func (r *StorageAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	return
}

// List the collection.
func (r *StorageAdapter) List(ctx *Context, provider *api.Provider) (itr fb.Iterator, err error) {
	storageName := fmt.Sprintf("Dummy storage for source provider %s", provider.Name)
	dummyStorge := Storage{
		Name: storageName,
		ID:   string(provider.UID),
	}
	list := fb.NewList()
	m := &model.Storage{
		Base: model.Base{
			ID:   dummyStorge.ID,
			Name: dummyStorge.Name,
		},
	}
	dummyStorge.ApplyTo(m)
	list.Append(m)

	itr = list.Iter()

	return
}

func (r *StorageAdapter) DeleteUnexisting(ctx *Context) (deletions []Updater, err error) {
	// Each provider have only one storage hence it can't be changed,
	// Will be removed only if the provider deleted.
	return
}
