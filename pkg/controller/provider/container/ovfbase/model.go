package ovfbase

import (
	"context"
	"errors"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovf"
	fb "github.com/kubev2v/forklift/pkg/lib/filebacked"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

// All adapters.
var adapterList []Adapter

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
	// Client.
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
	err = ctx.client.List("networks", &networkList)
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
	err = ctx.client.List("networks", &networkList)
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
	err = ctx.client.List("networks", &networkListServer)
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
			err = tx.Delete(m)
			if err != nil && errors.Is(err, libmodel.NotFound) {
				err = nil
			}
			return err
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
	err = ctx.client.List("vms", &vmList)
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
	err = ctx.client.List("vms", &vmList)
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
			} else if vm.OvfPath != m.OvfPath {
				vm.ApplyTo(m)
				err = tx.Update(m)
			} else if vm.ExportSource != m.ExportSource {
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
	err = ctx.client.List("vms", &vmListServer)
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
			err = tx.Delete(m)
			if err != nil && errors.Is(err, libmodel.NotFound) {
				err = nil
			}
			return err
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
	err = ctx.client.List("disks", &diskList)
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
	err = ctx.client.List("disks", &diskList)
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
	err = ctx.client.List("disks", &diskListServer)
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
			err = tx.Delete(m)
			if err != nil && errors.Is(err, libmodel.NotFound) {
				err = nil
			}
			return err
		}
		deletions = append(deletions, updater)
	}
	return
}

type StorageAdapter struct {
	BaseAdapter
}

func (r *StorageAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	disks := []Disk{}
	err = ctx.client.List("disks", &disks)
	if err != nil {
		return
	}
	for i := range disks {
		disk := &disks[i]
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Storage{
				Base: model.Base{
					ID: disk.ID,
				},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, libmodel.NotFound) {
					m.Name = disk.Name
					err = tx.Insert(m)
				}
				return
			}
			m.Name = disk.Name
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	return
}

// List the collection.
func (r *StorageAdapter) List(ctx *Context, provider *api.Provider) (itr fb.Iterator, err error) {
	diskList := []Disk{}
	err = ctx.client.List("disks", &diskList)
	if err != nil {
		return
	}
	list := fb.NewList()

	for _, object := range diskList {
		m := &model.Storage{
			Base: model.Base{
				ID:   object.ID,
				Name: object.Name,
			},
		}
		list.Append(m)
	}

	itr = list.Iter()

	return
}

func (r *StorageAdapter) DeleteUnexisting(ctx *Context) (deletions []Updater, err error) {
	storageList := []model.Storage{}
	err = ctx.db.List(&storageList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	inventory := make(map[string]bool)
	for _, storage := range storageList {
		inventory[storage.ID] = true
	}
	disks := []Disk{}
	err = ctx.client.List("disks", &disks)
	if err != nil {
		return
	}
	gone := []string{}
	for _, disk := range disks {
		if _, found := inventory[disk.ID]; !found {
			gone = append(gone, disk.ID)
		}
	}
	for _, id := range gone {
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Storage{
				Base: model.Base{ID: id},
			}
			err = tx.Delete(m)
			if err != nil && errors.Is(err, libmodel.NotFound) {
				err = nil
			}
			return err
		}
		deletions = append(deletions, updater)
	}
	return
}
