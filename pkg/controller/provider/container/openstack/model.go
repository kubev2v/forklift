package openstack

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/openstack"
	fb "github.com/konveyor/forklift-controller/pkg/lib/filebacked"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
)

// All adapters.
var adapterList []Adapter

func init() {
	adapterList = []Adapter{
		&RegionAdapter{},
		&ProjectAdapter{},
		&ImageAdapter{},
		&FlavorAdapter{},
		&VMAdapter{},
		&SnapshotAdapter{},
		&VolumeAdapter{},
		&VolumeTypeAdapter{},
		&NetworkAdapter{},
		&SubnetAdapter{},
	}
}

// Updates the DB based on
// changes described by an Event.
type Updater func(tx *libmodel.Tx) error

// Adapter context.
type Context struct {
	// Context.
	ctx context.Context
	// DB client.
	db libmodel.DB
	// OpenStack client.
	client *Client
	// Log.
	log logr.Logger
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
	// Get object updates
	GetUpdates(ctx *Context) (updates []Updater, err error)
	// Clean unexisting objects within the database
	DeleteUnexisting(ctx *Context) (updates []Updater, err error)
}

type RegionAdapter struct {
}

func (r *RegionAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &RegionListOpts{}
	regionList := []Region{}
	err = ctx.client.list(&regionList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, region := range regionList {
		m := &model.Region{
			Base: model.Base{ID: region.ID},
		}
		region.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

func (r *RegionAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	opts := &RegionListOpts{}
	regionList := []Region{}
	err = ctx.client.list(&regionList, opts)
	if err != nil {
		return
	}
	for i := range regionList {
		region := &regionList[i]
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Region{
				Base: model.Base{ID: region.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, libmodel.NotFound) {
					region.ApplyTo(m)
					err = tx.Insert(m)
				}
				return
			}
			if region.equalsTo(m) {
				return
			}
			region.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	return
}

func (r *RegionAdapter) DeleteUnexisting(ctx *Context) (updates []Updater, err error) {
	regionList := []model.Region{}
	err = ctx.db.List(&regionList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	for i := range regionList {
		region := &regionList[i]
		s := &Region{}
		err = ctx.client.get(s, region.ID)
		if err != nil && !ctx.client.isNotFound(err) {
			return
		}
		if ctx.client.isNotFound(err) {
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Region{
					Base: model.Base{ID: region.ID},
				}
				return tx.Delete(m)
			}
			updates = append(updates, updater)
			err = nil
		}
	}
	return
}

type ProjectAdapter struct {
}

func (r *ProjectAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &ProjectListOpts{}
	projectList := []Project{}
	err = ctx.client.list(&projectList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, project := range projectList {
		m := &model.Project{
			Base: model.Base{ID: project.ID},
		}
		project.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

func (r *ProjectAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	opts := &ProjectListOpts{}
	projectList := []Project{}
	err = ctx.client.list(&projectList, opts)
	if err != nil {
		return
	}
	for i := range projectList {
		project := &projectList[i]
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Project{
				Base: model.Base{ID: project.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, libmodel.NotFound) {
					project.ApplyTo(m)
					err = tx.Insert(m)
				}
				return
			}
			if project.equalsTo(m) {
				return
			}
			project.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	return
}

func (r *ProjectAdapter) DeleteUnexisting(ctx *Context) (updates []Updater, err error) {
	projectList := []model.Project{}
	err = ctx.db.List(&projectList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	for i := range projectList {
		project := &projectList[i]
		s := &Project{}
		err = ctx.client.get(s, project.ID)
		if err != nil && !ctx.client.isNotFound(err) {
			return
		}
		if ctx.client.isNotFound(err) {
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Project{
					Base: model.Base{ID: project.ID},
				}
				return tx.Delete(m)
			}
			updates = append(updates, updater)
			err = nil
		}
	}
	return
}

type ImageAdapter struct {
	lastSync time.Time
}

func (r *ImageAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &ImageListOpts{}
	imageList := []Image{}
	// Set time to epoch start
	updateTime := time.Unix(0, 0)
	err = ctx.client.list(&imageList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, image := range imageList {
		m := &model.Image{
			Base: model.Base{ID: image.ID},
		}
		image.ApplyTo(m)
		list.Append(m)
		if image.UpdatedAt.After(updateTime) {
			updateTime = image.UpdatedAt
		}
	}
	itr = list.Iter()
	r.lastSync = updateTime
	return
}

func (r *ImageAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	opts := &ImageListOpts{}
	opts.setUpdateAtQueryFilterGT(r.lastSync)
	imageList := []Image{}
	// Set time to epoch start
	updateTime := time.Unix(0, 0)
	err = ctx.client.list(&imageList, opts)
	if err != nil {
		return
	}
	for i := range imageList {
		image := &imageList[i]
		switch image.Status {
		case ImageStatusDeleted, ImageStatusPendingDelete:
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Image{
					Base: model.Base{ID: image.ID},
				}
				image.ApplyTo(m)
				err = tx.Delete(m)
				return
			}
			updates = append(updates, updater)

		default:
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Image{
					Base: model.Base{ID: image.ID},
				}
				err = tx.Get(m)
				if err != nil {
					if errors.Is(err, libmodel.NotFound) {
						image.ApplyTo(m)
						err = tx.Insert(m)
					}
					return
				}
				if !image.updatedAfter(m) {
					return
				}
				image.ApplyTo(m)
				err = tx.Update(m)
				return
			}
			updates = append(updates, updater)
		}

		if image.UpdatedAt.After(updateTime) {
			updateTime = image.UpdatedAt
		}
	}
	r.lastSync = updateTime
	return
}

func (r *ImageAdapter) DeleteUnexisting(ctx *Context) (updates []Updater, err error) {
	imageList := []model.Image{}
	err = ctx.db.List(&imageList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	for i := range imageList {
		image := &imageList[i]
		s := &Image{}
		err = ctx.client.get(s, image.ID)
		if err != nil && !ctx.client.isNotFound(err) {
			return
		}
		if ctx.client.isNotFound(err) {
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Image{
					Base: model.Base{ID: image.ID},
				}
				return tx.Delete(m)
			}
			updates = append(updates, updater)
			err = nil
		}
	}
	return
}

type FlavorAdapter struct {
	lastSync time.Time
}

func (r *FlavorAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &FlavorListOpts{}
	flavorList := []Flavor{}
	now := time.Now()
	err = ctx.client.list(&flavorList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, flavor := range flavorList {
		m := &model.Flavor{
			Base: model.Base{ID: flavor.ID},
		}
		flavor.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()
	r.lastSync = now
	return
}

func (r *FlavorAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	opts := &FlavorListOpts{}
	opts.ChangesSince = r.lastSync.Format(time.RFC3339)
	flavorList := []Flavor{}
	now := time.Now()
	err = ctx.client.list(&flavorList, opts)
	if err != nil {
		return
	}
	for i := range flavorList {
		flavor := &flavorList[i]
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Flavor{
				Base: model.Base{ID: flavor.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, libmodel.NotFound) {
					flavor.ApplyTo(m)
					err = tx.Insert(m)
				}
				return
			}
			if flavor.equalsTo(m) {
				return
			}
			flavor.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	r.lastSync = now
	return
}

func (r *FlavorAdapter) DeleteUnexisting(ctx *Context) (updates []Updater, err error) {
	flavorList := []model.Flavor{}
	err = ctx.db.List(&flavorList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	for i := range flavorList {
		flavor := &flavorList[i]
		s := &Flavor{}
		err = ctx.client.get(s, flavor.ID)
		if err != nil && !ctx.client.isNotFound(err) {
			return
		}
		if ctx.client.isNotFound(err) {
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Flavor{
					Base: model.Base{ID: flavor.ID},
				}
				return tx.Delete(m)
			}
			updates = append(updates, updater)
			err = nil
		}
	}
	return
}

// VM adapter.
type VMAdapter struct {
	lastSync time.Time
}

// List the collection.
func (r *VMAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &VMListOpts{}
	vmList := []VM{}
	// Set time to epoch start
	updateTime := time.Unix(0, 0)
	err = ctx.client.list(&vmList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, server := range vmList {
		m := &model.VM{
			Base: model.Base{
				ID:   server.ID,
				Name: server.Name},
		}
		server.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()
	r.lastSync = updateTime
	return
}

// Get updates since last sync.
func (r *VMAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	opts := &VMListOpts{}
	opts.ChangesSince = r.lastSync.Format(time.RFC3339)
	vmList := []VM{}
	// Set time to epoch start
	updateTime := time.Unix(0, 0)
	err = ctx.client.list(&vmList, opts)
	if err != nil {
		return
	}
	for i := range vmList {
		vm := &vmList[i]
		switch vm.Status {
		case model.VmStatusDeleted, model.VmStatusSoftDeleted:
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.VM{
					Base: model.Base{ID: vm.ID},
				}
				vm.ApplyTo(m)
				err = tx.Delete(m)
				if errors.Is(err, libmodel.NotFound) {
					err = nil
				}
				return
			}
			updates = append(updates, updater)

		default:
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.VM{
					Base: model.Base{ID: vm.ID},
				}
				err = tx.Get(m)
				if err != nil {
					if errors.Is(err, libmodel.NotFound) {
						vm.ApplyTo(m)
						err = tx.Insert(m)
					}
					return
				}
				if vm.equalsTo(m) {
					return
				}
				vm.ApplyTo(m)
				err = tx.Update(m)
				return
			}
			updates = append(updates, updater)
		}
		if vm.Updated.After(updateTime) {
			updateTime = vm.Updated
		}
	}
	r.lastSync = updateTime
	return
}

func (r *VMAdapter) DeleteUnexisting(ctx *Context) (updates []Updater, err error) {
	vmList := []model.VM{}
	err = ctx.db.List(&vmList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	for i := range vmList {
		vm := &vmList[i]
		s := &VM{}
		err = ctx.client.get(s, vm.ID)
		if err != nil && !ctx.client.isNotFound(err) {
			return
		}
		if ctx.client.isNotFound(err) {
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.VM{
					Base: model.Base{ID: vm.ID},
				}
				return tx.Delete(m)
			}
			updates = append(updates, updater)
			err = nil
		}
	}
	return
}

type SnapshotAdapter struct {
}

func (r *SnapshotAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	snapshotList := []Snapshot{}
	err = ctx.client.list(&snapshotList, nil)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, snapshot := range snapshotList {
		m := &model.Snapshot{
			Base: model.Base{ID: snapshot.ID},
		}
		snapshot.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

func (r *SnapshotAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	snapshotList := []Snapshot{}
	err = ctx.client.list(&snapshotList, nil)
	if err != nil {
		return
	}
	for i := range snapshotList {
		snapshot := &snapshotList[i]
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Snapshot{
				Base: model.Base{ID: snapshot.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, libmodel.NotFound) {
					snapshot.ApplyTo(m)
					err = tx.Insert(m)
				}
				return
			}
			if !snapshot.updatedAfter(m) {
				return
			}
			snapshot.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}

	return
}

func (r *SnapshotAdapter) DeleteUnexisting(ctx *Context) (updates []Updater, err error) {
	snapshotList := []model.Snapshot{}
	err = ctx.db.List(&snapshotList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	for i := range snapshotList {
		snapshot := &snapshotList[i]
		s := &Snapshot{}
		err = ctx.client.get(s, snapshot.ID)
		if err != nil && !ctx.client.isNotFound(err) {
			return
		}
		if ctx.client.isNotFound(err) {
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Snapshot{
					Base: model.Base{ID: snapshot.ID},
				}
				return tx.Delete(m)
			}
			updates = append(updates, updater)
			err = nil
		}
	}
	return
}

type VolumeAdapter struct {
}

func (r *VolumeAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &VolumeListOpts{}
	volumeList := []Volume{}
	err = ctx.client.list(&volumeList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, volume := range volumeList {
		m := &model.Volume{
			Base: model.Base{ID: volume.ID},
		}
		volume.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

// UpdatedAt volume list options not imlemented yet in gophercloud
func (r *VolumeAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	opts := &VolumeListOpts{}
	volumeList := []Volume{}
	err = ctx.client.list(&volumeList, opts)
	if err != nil {
		return
	}
	for i := range volumeList {
		volume := &volumeList[i]
		switch volume.Status {
		case VolumeStatusDeleting:
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Volume{
					Base: model.Base{ID: volume.ID},
				}
				volume.ApplyTo(m)
				err = tx.Delete(m)
				if errors.Is(err, libmodel.NotFound) {
					err = nil
				}
				return
			}
			updates = append(updates, updater)

		default:
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Volume{
					Base: model.Base{ID: volume.ID},
				}
				err = tx.Get(m)
				if err != nil {
					if errors.Is(err, libmodel.NotFound) {
						volume.ApplyTo(m)
						err = tx.Insert(m)
					}
					return
				}
				if !volume.updatedAfter(m) {
					return
				}
				volume.ApplyTo(m)
				err = tx.Update(m)
				return
			}
			updates = append(updates, updater)
		}
	}
	return
}

func (r *VolumeAdapter) DeleteUnexisting(ctx *Context) (updates []Updater, err error) {
	volumeList := []model.Volume{}
	err = ctx.db.List(&volumeList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	for i := range volumeList {
		volume := &volumeList[i]
		s := &Volume{}
		err = ctx.client.get(s, volume.ID)
		if err != nil && !ctx.client.isNotFound(err) {
			return
		}
		if ctx.client.isNotFound(err) {
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Volume{
					Base: model.Base{ID: volume.ID},
				}
				return tx.Delete(m)
			}
			updates = append(updates, updater)
			err = nil
		}
	}
	return
}

type VolumeTypeAdapter struct {
}

func (r *VolumeTypeAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &VolumeTypeListOpts{}
	volumeTypeList := []VolumeType{}
	err = ctx.client.list(&volumeTypeList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, volumeType := range volumeTypeList {
		m := &model.VolumeType{
			Base: model.Base{ID: volumeType.ID},
		}
		volumeType.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

// UpdatedAt volume list options not imlemented yet in gophercloud
func (r *VolumeTypeAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	opts := &VolumeTypeListOpts{}
	volumeTypeList := []VolumeType{}
	err = ctx.client.list(&volumeTypeList, opts)
	if err != nil {
		return
	}
	for i := range volumeTypeList {
		volumeType := &volumeTypeList[i]
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.VolumeType{
				Base: model.Base{ID: volumeType.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, libmodel.NotFound) {
					volumeType.ApplyTo(m)
					err = tx.Insert(m)
				}
				return
			}
			if volumeType.equalsTo(m) {
				return
			}
			volumeType.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	return
}

func (r *VolumeTypeAdapter) DeleteUnexisting(ctx *Context) (updates []Updater, err error) {
	volumeTypeList := []model.VolumeType{}
	err = ctx.db.List(&volumeTypeList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	for i := range volumeTypeList {
		volumeType := &volumeTypeList[i]
		s := &VolumeType{}
		err = ctx.client.get(s, volumeType.ID)
		if err != nil && !ctx.client.isNotFound(err) {
			return
		}
		if ctx.client.isNotFound(err) {
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.VolumeType{
					Base: model.Base{ID: volumeType.ID},
				}
				return tx.Delete(m)
			}
			updates = append(updates, updater)
			err = nil
		}
	}
	return
}

type NetworkAdapter struct {
}

func (r *NetworkAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	networkList := []Network{}
	opts := &NetworkListOpts{}
	err = ctx.client.list(&networkList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, network := range networkList {
		m := &model.Network{
			Base: model.Base{ID: network.ID},
		}
		network.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

func (r *NetworkAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	networkList := []Network{}
	opts := &NetworkListOpts{}
	err = ctx.client.list(&networkList, opts)
	if err != nil {
		return
	}
	for i := range networkList {
		network := &networkList[i]
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Network{
				Base: model.Base{ID: network.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, libmodel.NotFound) {
					network.ApplyTo(m)
					err = tx.Insert(m)
				}
				return
			}
			if !network.updatedAfter(m) {
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

func (r *NetworkAdapter) DeleteUnexisting(ctx *Context) (updates []Updater, err error) {
	networkList := []model.Network{}
	err = ctx.db.List(&networkList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	for i := range networkList {
		network := &networkList[i]
		s := &Network{}
		err = ctx.client.get(s, network.ID)
		if err != nil && !ctx.client.isNotFound(err) {
			return
		}
		if ctx.client.isNotFound(err) {
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Network{
					Base: model.Base{ID: network.ID},
				}
				return tx.Delete(m)
			}
			updates = append(updates, updater)
			err = nil
		}
	}
	return
}

type SubnetAdapter struct {
}

func (r *SubnetAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	subnetList := []Subnet{}
	opts := &SubnetListOpts{}
	err = ctx.client.list(&subnetList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, subnet := range subnetList {
		m := &model.Subnet{
			Base: model.Base{ID: subnet.ID},
		}
		subnet.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

func (r *SubnetAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	subnetList := []Subnet{}
	opts := &SubnetListOpts{}
	err = ctx.client.list(&subnetList, opts)
	if err != nil {
		return
	}
	for i := range subnetList {
		subnet := &subnetList[i]
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Subnet{
				Base: model.Base{ID: subnet.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, libmodel.NotFound) {
					subnet.ApplyTo(m)
					err = tx.Insert(m)
				}
				return
			}
			if subnet.equalsTo(m) {
				return
			}
			subnet.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	return
}

func (r *SubnetAdapter) DeleteUnexisting(ctx *Context) (updates []Updater, err error) {
	subnetList := []model.Subnet{}
	err = ctx.db.List(&subnetList, libmodel.FilterOptions{})
	if err != nil {
		if errors.Is(err, libmodel.NotFound) {
			err = nil
		}
		return
	}
	for i := range subnetList {
		subnet := &subnetList[i]
		s := &Subnet{}
		err = ctx.client.get(s, subnet.ID)
		if err != nil && !ctx.client.isNotFound(err) {
			return
		}
		if ctx.client.isNotFound(err) {
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Subnet{
					Base: model.Base{ID: subnet.ID},
				}
				return tx.Delete(m)
			}
			updates = append(updates, updater)
			err = nil
		}
	}
	return
}
