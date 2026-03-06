package hyperv

import (
	"errors"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	hvutil "github.com/kubev2v/forklift/pkg/controller/hyperv"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	types "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv/types"
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
	networkList, err := ctx.client.ListNetworks()
	if err != nil {
		return
	}
	list := fb.NewList()
	for i := range networkList {
		m := &model.Network{}
		applyNetworkTo(&networkList[i], m)
		list.Append(m)
	}
	itr = list.Iter()
	return
}

func (r *NetworkAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	networkList, err := ctx.client.ListNetworks()
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
					applyNetworkTo(network, m)
					err = tx.Insert(m)
				}
				return
			}
			applyNetworkTo(network, m)
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
	serverList, err := ctx.client.ListNetworks()
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
	storageList, err := ctx.client.ListStorages()
	if err != nil {
		return
	}
	list := fb.NewList()
	for i := range storageList {
		m := &model.Storage{}
		applyStorageTo(&storageList[i], m)
		list.Append(m)
	}
	itr = list.Iter()
	return
}

func (r *StorageAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	storageList, err := ctx.client.ListStorages()
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
					applyStorageTo(stor, m)
					err = tx.Insert(m)
				}
				return
			}
			applyStorageTo(stor, m)
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
	serverList, err := ctx.client.ListStorages()
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
	diskList, err := ctx.client.ListDisks()
	if err != nil {
		return
	}
	list := fb.NewList()
	for i := range diskList {
		m := &model.Disk{}
		applyDiskTo(&diskList[i], m)
		list.Append(m)
	}
	itr = list.Iter()
	return
}

func (r *DiskAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	diskList, err := ctx.client.ListDisks()
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
					applyDiskTo(disk, m)
					err = tx.Insert(m)
				}
				return
			}
			applyDiskTo(disk, m)
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
	serverList, err := ctx.client.ListDisks()
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
	vmList, err := ctx.client.ListVMs()
	if err != nil {
		return
	}
	list := fb.NewList()
	for i := range vmList {
		m := &model.VM{}
		applyVMTo(&vmList[i], m)
		list.Append(m)
	}
	itr = list.Iter()
	return
}

// Get updates since last sync.
func (r *VMAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	vmList, err := ctx.client.ListVMs()
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
					applyVMTo(vm, m)
					err = tx.Insert(m)
				}
				return
			}
			// Preserve GuestNetworks if VM is now off but had data before
			// (KVP Exchange only works when VM is running)
			existingGuestNetworks := m.GuestNetworks
			existingGuestOS := m.GuestOS
			applyVMTo(vm, m)
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
	serverList, err := ctx.client.ListVMs()
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

// Apply VM to (update) the model.
func applyVMTo(r *types.VM, m *model.VM) {
	m.ID = r.UUID
	m.Name = r.Name
	m.UUID = r.UUID
	m.PowerState = r.PowerState
	m.CpuCount = int32(r.CpuCount)
	m.MemoryMB = int32(r.MemoryMB)
	m.Firmware = r.Firmware
	m.GuestOS = r.GuestOS
	m.TpmEnabled = r.TpmEnabled
	m.SecureBoot = r.SecureBoot
	m.HasCheckpoint = r.HasCheckpoint
	addVMDisks(r, m)
	addVMNICs(r, m)
	addVMGuestNetworks(r, m)
	addVMConcerns(r, m)
	SortNICsByGuestNetworkOrder(m)
}

func addVMDisks(r *types.VM, m *model.VM) {
	m.Disks = nil
	for _, d := range r.Disks {
		diskName := d.ID
		if d.WindowsPath != "" {
			parts := strings.Split(d.WindowsPath, "\\")
			if len(parts) > 0 {
				diskName = parts[len(parts)-1]
			}
		}
		m.Disks = append(m.Disks, model.Disk{
			Base: model.Base{
				ID:   d.ID,
				Name: diskName,
			},
			WindowsPath: d.WindowsPath,
			SMBPath:     d.SMBPath,
			Datastore: model.Ref{
				Kind: "Storage",
				ID:   hvutil.StorageIDDefault,
			},
			Capacity:   d.Capacity,
			Format:     d.Format,
			RCTEnabled: d.RCTEnabled,
		})
	}
}

func addVMNICs(r *types.VM, m *model.VM) {
	m.NICs = nil
	networkSet := make(map[string]bool)
	for _, n := range r.NICs {
		m.NICs = append(m.NICs, model.NIC{
			Name:        n.Name,
			MAC:         n.MAC,
			DeviceIndex: n.DeviceIndex,
			Network: model.Ref{
				Kind: "Network",
				ID:   n.NetworkUUID,
			},
			NetworkName: n.NetworkName,
		})
		if n.NetworkUUID != "" {
			networkSet[n.NetworkUUID] = true
		}
	}
	m.Networks = nil
	for uuid := range networkSet {
		m.Networks = append(m.Networks, model.Ref{
			Kind: "Network",
			ID:   uuid,
		})
	}
}

func addVMGuestNetworks(r *types.VM, m *model.VM) {
	m.GuestNetworks = nil
	for _, gn := range r.GuestNetworks {
		m.GuestNetworks = append(m.GuestNetworks, model.GuestNetwork{
			MAC:          gn.MAC,
			IP:           gn.IP,
			DeviceIndex:  gn.DeviceIndex,
			Origin:       gn.Origin,
			PrefixLength: gn.PrefixLength,
			DNS:          gn.DNS,
			Gateway:      gn.Gateway,
		})
	}
}

func addVMConcerns(r *types.VM, m *model.VM) {
	m.Concerns = nil
	for _, c := range r.Concerns {
		m.Concerns = append(m.Concerns, model.Concern{
			Category:   c.Category,
			Label:      c.Label,
			Assessment: c.Message,
		})
	}
}

// Apply Disk to (update) the model.
func applyDiskTo(r *types.Disk, m *model.Disk) {
	diskName := r.ID
	if r.WindowsPath != "" {
		parts := strings.Split(r.WindowsPath, "\\")
		if len(parts) > 0 {
			diskName = parts[len(parts)-1]
		}
	}
	m.ID = r.ID
	m.Name = diskName
	m.WindowsPath = r.WindowsPath
	m.SMBPath = r.SMBPath
	m.Capacity = r.Capacity
	m.Format = r.Format
	m.RCTEnabled = r.RCTEnabled
	m.Datastore = model.Ref{
		Kind: "Storage",
		ID:   hvutil.StorageIDDefault,
	}
}

// Apply Network to (update) the model.
func applyNetworkTo(r *types.Network, m *model.Network) {
	m.ID = r.UUID
	m.Name = r.Name
	m.UUID = r.UUID
	m.SwitchName = r.Name
	m.SwitchType = r.SwitchType
}

// Apply Storage to (update) the model.
func applyStorageTo(r *types.Storage, m *model.Storage) {
	m.ID = r.ID
	m.Name = r.Name
	m.Type = r.Type
	m.Path = r.Path
	m.Capacity = r.Capacity
	m.Free = r.Free
}
