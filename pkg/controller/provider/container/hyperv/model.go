package hyperv

import (
	"errors"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	fb "github.com/kubev2v/forklift/pkg/lib/filebacked"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// All adapters.
var adapterList []Adapter

func init() {
	adapterList = []Adapter{
		&NetworkAdapter{},
		&StorageAdapter{},
		&DiskAdapter{},
		&VMAdapter{},
	}
}

// Updates the DB based on changes described by an Event.
type Updater func(tx *libmodel.Tx) error

// Model adapter.
// Provides integration between the REST resource
// model and the inventory model.
type Adapter interface {
	// List REST collections.
	List(ctx *Context, provider *api.Provider) (itr fb.Iterator, err error)
	// Get object updates.
	GetUpdates(ctx *Context) (updater []Updater, err error)
	// Clean unexisting objects within the database.
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
	for i := range networkList {
		m := &model.Network{}
		networkList[i].ApplyTo(m)
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
				Base: model.Base{ID: network.UUID},
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
	existing := []model.Network{}
	err = ctx.db.List(&existing, libmodel.ListOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	serverList := []Network{}
	err = ctx.client.List("networks", &serverList)
	if err != nil {
		return
	}
	serverMap := make(map[string]bool)
	for _, n := range serverList {
		serverMap[n.UUID] = true
	}
	for _, n := range existing {
		if _, found := serverMap[n.ID]; !found {
			currentID := n.ID
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Network{Base: model.Base{ID: currentID}}
				err = tx.Delete(m)
				if err != nil && errors.Is(err, libmodel.NotFound) {
					err = nil
				}
				return
			}
			deletions = append(deletions, updater)
		}
	}
	return
}

// Storage adapter.
type StorageAdapter struct {
	BaseAdapter
}

// List the collection.
func (r *StorageAdapter) List(ctx *Context, provider *api.Provider) (itr fb.Iterator, err error) {
	storageList := []Storage{}
	err = ctx.client.List("storages", &storageList)
	if err != nil {
		return
	}
	list := fb.NewList()
	for i := range storageList {
		m := &model.Storage{}
		storageList[i].ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()
	return
}

func (r *StorageAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	storageList := []Storage{}
	err = ctx.client.List("storages", &storageList)
	if err != nil {
		return
	}
	for i := range storageList {
		stor := &storageList[i]
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Storage{
				Base: model.Base{ID: stor.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, libmodel.NotFound) {
					stor.ApplyTo(m)
					err = tx.Insert(m)
				}
				return
			}
			stor.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	return
}

func (r *StorageAdapter) DeleteUnexisting(ctx *Context) (deletions []Updater, err error) {
	existing := []model.Storage{}
	err = ctx.db.List(&existing, libmodel.ListOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	serverList := []Storage{}
	err = ctx.client.List("storages", &serverList)
	if err != nil {
		return
	}
	serverMap := make(map[string]bool)
	for _, s := range serverList {
		serverMap[s.ID] = true
	}
	for _, s := range existing {
		if _, found := serverMap[s.ID]; !found {
			currentID := s.ID
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Storage{Base: model.Base{ID: currentID}}
				err = tx.Delete(m)
				if err != nil && errors.Is(err, libmodel.NotFound) {
					err = nil
				}
				return
			}
			deletions = append(deletions, updater)
		}
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
	for i := range diskList {
		m := &model.Disk{}
		diskList[i].ApplyTo(m)
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
				Base: model.Base{ID: disk.ID},
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
	existing := []model.Disk{}
	err = ctx.db.List(&existing, libmodel.ListOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	serverList := []Disk{}
	err = ctx.client.List("disks", &serverList)
	if err != nil {
		return
	}
	serverMap := make(map[string]bool)
	for _, d := range serverList {
		serverMap[d.ID] = true
	}
	for _, d := range existing {
		if _, found := serverMap[d.ID]; !found {
			currentID := d.ID
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Disk{Base: model.Base{ID: currentID}}
				err = tx.Delete(m)
				if err != nil && errors.Is(err, libmodel.NotFound) {
					err = nil
				}
				return
			}
			deletions = append(deletions, updater)
		}
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
	for i := range vmList {
		m := &model.VM{}
		vmList[i].ApplyTo(m)
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
				return
			}
			// Preserve GuestNetworks if VM is now off but had data before
			// (KVP Exchange only works when VM is running)
			existingGuestNetworks := m.GuestNetworks
			existingGuestOS := m.GuestOS
			vm.ApplyTo(m)
			if len(m.GuestNetworks) == 0 && len(existingGuestNetworks) > 0 {
				m.GuestNetworks = existingGuestNetworks
			}
			if m.GuestOS == "" && existingGuestOS != "" {
				m.GuestOS = existingGuestOS
			}
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	return
}

func (r *VMAdapter) DeleteUnexisting(ctx *Context) (deletions []Updater, err error) {
	existing := []model.VM{}
	err = ctx.db.List(&existing, libmodel.ListOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	serverList := []VM{}
	err = ctx.client.List("vms", &serverList)
	if err != nil {
		return
	}
	serverMap := make(map[string]bool)
	for _, vm := range serverList {
		serverMap[vm.UUID] = true
	}
	for _, vm := range existing {
		if _, found := serverMap[vm.ID]; !found {
			currentID := vm.ID
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.VM{Base: model.Base{ID: currentID}}
				err = tx.Delete(m)
				if err != nil && errors.Is(err, libmodel.NotFound) {
					err = nil
				}
				return
			}
			deletions = append(deletions, updater)
		}
	}
	return
}
