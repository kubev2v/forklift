package openstack

import (
	"context"
	"errors"
	"time"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/openstack"
	libclient "github.com/kubev2v/forklift/pkg/lib/client/openstack"
	fb "github.com/kubev2v/forklift/pkg/lib/filebacked"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
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
	// Get object updates
	GetUpdates(ctx *Context) (updates []Updater, err error)
	// Clean unexisting objects within the database
	DeleteUnexisting(ctx *Context) (updates []Updater, err error)
}

type RegionAdapter struct {
}

func (r *RegionAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	regionList := []libclient.Region{}
	region := &libclient.Region{}
	err = ctx.client.GetClientRegion(region)
	if err != nil {
		return
	}
	regionList = append(regionList, *region)

	list := fb.NewList()
	if err != nil {
		return
	}
	for _, region := range regionList {
		m := &model.Region{
			Base: model.Base{ID: region.ID},
		}
		r := &Region{region}
		r.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

func (r *RegionAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	regionList := []libclient.Region{}
	region := &libclient.Region{}
	err = ctx.client.GetClientRegion(region)
	if err != nil {
		return
	}
	regionList = append(regionList, *region)
	if err != nil {
		return
	}
	for i := range regionList {
		region := &Region{regionList[i]}
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
		clientRegion := &libclient.Region{}
		err = ctx.client.Get(clientRegion, region.ID)
		if err != nil {
			if ctx.client.IsNotFound(err) {
				updater := func(tx *libmodel.Tx) (err error) {
					m := &model.Region{
						Base: model.Base{ID: region.ID},
					}
					return tx.Delete(m)
				}
				updates = append(updates, updater)
				err = nil
			} else {
				return
			}
		}
	}
	return
}

type ProjectAdapter struct {
}

func (r *ProjectAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	projectList := []libclient.Project{}
	clientProject := &libclient.Project{}
	err = ctx.client.GetClientProject(clientProject)
	if err != nil {
		return
	}
	projectList = append(projectList, *clientProject)
	list := fb.NewList()
	for _, project := range projectList {
		m := &model.Project{
			Base: model.Base{ID: project.ID},
		}
		p := &Project{project}
		p.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

func (r *ProjectAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	projectList := []libclient.Project{}
	clientProject := &libclient.Project{}
	err = ctx.client.GetClientProject(clientProject)
	if err != nil {
		return
	}
	projectList = append(projectList, *clientProject)
	for i := range projectList {
		project := &Project{projectList[i]}
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
		clientProject := &libclient.Project{}
		err = ctx.client.Get(clientProject, project.ID)
		if err != nil {
			if ctx.client.IsNotFound(err) {
				updater := func(tx *libmodel.Tx) (err error) {
					m := &model.Project{
						Base: model.Base{ID: project.ID},
					}
					return tx.Delete(m)
				}
				updates = append(updates, updater)
				err = nil
			} else {
				return
			}
		}
	}
	return
}

type ImageAdapter struct {
	lastSync time.Time
}

func (r *ImageAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &libclient.ImageListOpts{}
	imageList := []libclient.Image{}
	// Set time to epoch start
	updateTime := time.Unix(0, 0)
	err = ctx.client.List(&imageList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, image := range imageList {
		m := &model.Image{
			Base: model.Base{ID: image.ID},
		}
		i := &Image{image}
		i.ApplyTo(m)
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
	opts := &libclient.ImageListOpts{}
	opts.SetUpdateAtQueryFilterGT(r.lastSync)
	imageList := []libclient.Image{}
	// Set time to epoch start
	updateTime := time.Unix(0, 0)
	err = ctx.client.List(&imageList, opts)
	if err != nil {
		return
	}
	for i := range imageList {
		image := &Image{imageList[i]}
		switch image.Status {
		case libclient.ImageStatusDeleted, libclient.ImageStatusPendingDelete:
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
		s := &libclient.Image{}
		err = ctx.client.Get(s, image.ID)
		if err != nil {
			if ctx.client.IsNotFound(err) {
				updater := func(tx *libmodel.Tx) (err error) {
					m := &model.Image{
						Base: model.Base{ID: image.ID},
					}
					return tx.Delete(m)
				}
				updates = append(updates, updater)
				err = nil
			} else {
				return
			}
		}
	}
	return
}

type FlavorAdapter struct {
	lastSync time.Time
}

func (r *FlavorAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &libclient.FlavorListOpts{}
	flavorList := []libclient.Flavor{}
	now := time.Now()
	err = ctx.client.List(&flavorList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, flavor := range flavorList {
		m := &model.Flavor{
			Base: model.Base{ID: flavor.ID},
		}
		f := &Flavor{Flavor: flavor, ExtraSpecs: flavor.ExtraSpecs}
		f.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()
	r.lastSync = now
	return
}

func (r *FlavorAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	opts := &libclient.FlavorListOpts{}
	opts.ChangesSince = r.lastSync.Format(time.RFC3339)
	flavorList := []libclient.Flavor{}
	now := time.Now()
	err = ctx.client.List(&flavorList, opts)
	if err != nil {
		return
	}
	for i := range flavorList {
		flavor := &Flavor{Flavor: flavorList[i], ExtraSpecs: flavorList[i].ExtraSpecs}
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
		s := &libclient.Flavor{}
		err = ctx.client.Get(s, flavor.ID)
		if err != nil {
			if ctx.client.IsNotFound(err) {
				updater := func(tx *libmodel.Tx) (err error) {
					m := &model.Flavor{
						Base: model.Base{ID: flavor.ID},
					}
					return tx.Delete(m)
				}
				updates = append(updates, updater)
				err = nil
			} else {
				return
			}
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
	opts := &libclient.VMListOpts{}
	vmList := []libclient.VM{}
	// Set time to epoch start
	updateTime := time.Unix(0, 0)
	err = ctx.client.List(&vmList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, vm := range vmList {
		m := &model.VM{
			Base: model.Base{
				ID:   vm.ID,
				Name: vm.Name},
		}
		v := &VM{vm}
		v.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()
	r.lastSync = updateTime
	return
}

// Get updates since last sync.
func (r *VMAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	opts := &libclient.VMListOpts{}
	opts.ChangesSince = r.lastSync.Format(time.RFC3339)
	vmList := []libclient.VM{}
	// Set time to epoch start
	updateTime := time.Unix(0, 0)
	err = ctx.client.List(&vmList, opts)
	if err != nil {
		return
	}
	for i := range vmList {
		vm := &VM{vmList[i]}
		switch vm.Status {
		case libclient.VmStatusDeleted, libclient.VmStatusSoftDeleted:
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
				if err = tx.Get(m); err != nil {
					if errors.Is(err, libmodel.NotFound) {
						vm.ApplyTo(m)
						err = tx.Insert(m)
					}
				} else if !vm.equalsTo(m) {
					vm.ApplyTo(m)
					err = tx.Update(m)
				}
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
		s := &libclient.VM{}
		err = ctx.client.Get(s, vm.ID)
		if err != nil {
			if ctx.client.IsNotFound(err) {
				updater := func(tx *libmodel.Tx) (err error) {
					m := &model.VM{
						Base: model.Base{ID: vm.ID},
					}
					return tx.Delete(m)
				}
				updates = append(updates, updater)
				err = nil
			} else {
				return
			}
		}
	}
	return
}

type SnapshotAdapter struct {
}

func (r *SnapshotAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	snapshotList := []libclient.Snapshot{}
	opts := &libclient.SnapshotListOpts{}
	err = ctx.client.List(&snapshotList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, snapshot := range snapshotList {
		m := &model.Snapshot{
			Base: model.Base{ID: snapshot.ID},
		}
		s := &Snapshot{snapshot}
		s.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

func (r *SnapshotAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	snapshotList := []libclient.Snapshot{}
	opts := &libclient.SnapshotListOpts{}
	err = ctx.client.List(&snapshotList, opts)
	if err != nil {
		return
	}
	for i := range snapshotList {
		snapshot := &Snapshot{snapshotList[i]}
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
		s := &libclient.Snapshot{}
		err = ctx.client.Get(s, snapshot.ID)
		if err != nil {
			if ctx.client.IsNotFound(err) {
				updater := func(tx *libmodel.Tx) (err error) {
					m := &model.Snapshot{
						Base: model.Base{ID: snapshot.ID},
					}
					return tx.Delete(m)
				}
				updates = append(updates, updater)
				err = nil
			} else {
				return
			}
		}
	}
	return
}

type VolumeAdapter struct {
}

func (r *VolumeAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &libclient.VolumeListOpts{}
	volumeList := []libclient.Volume{}
	err = ctx.client.List(&volumeList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, volume := range volumeList {
		m := &model.Volume{
			Base: model.Base{ID: volume.ID},
		}
		v := &Volume{volume}
		v.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

// UpdatedAt volume list options not imlemented yet in gophercloud
func (r *VolumeAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	opts := &libclient.VolumeListOpts{}
	volumeList := []libclient.Volume{}
	err = ctx.client.List(&volumeList, opts)
	if err != nil {
		return
	}
	for i := range volumeList {
		volume := &Volume{volumeList[i]}
		ctx.log.Info("Getting update for volume", "volume", volume.ID)
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
				if err == nil {
					// If an attached volume has changed, we have to update the relevant VM revision
					// to make sure it is revalidated.
					ctx.log.Info("Volume changed, updating attached VMs", "volume", volume.ID)
					for _, attachment := range volume.Attachments {
						vmID := attachment.ServerID
						vm := &model.VM{
							Base: model.Base{ID: vmID},
						}
						err = tx.Get(vm)
						if err != nil {
							ctx.log.Info("VM not found, skipping", "vmID", vmID)
							continue
						}
						vm.RevisionValidated = 0
						err = tx.Update(vm)
						if err != nil {
							ctx.log.Error(err, "Could not update VM revision", "vmID", vmID)
							continue
						}
					}
				}

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
		s := &libclient.Volume{}
		err = ctx.client.Get(s, volume.ID)
		if err != nil {
			if ctx.client.IsNotFound(err) {
				updater := func(tx *libmodel.Tx) (err error) {
					m := &model.Volume{
						Base: model.Base{ID: volume.ID},
					}
					return tx.Delete(m)
				}
				updates = append(updates, updater)
				err = nil
			} else {
				return
			}
		}
	}
	return
}

type VolumeTypeAdapter struct {
}

func (r *VolumeTypeAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &libclient.VolumeTypeListOpts{}
	volumeTypeList := []libclient.VolumeType{}
	err = ctx.client.List(&volumeTypeList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, volumeType := range volumeTypeList {
		m := &model.VolumeType{
			Base: model.Base{ID: volumeType.ID},
		}
		v := &VolumeType{volumeType}
		v.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

// UpdatedAt volume list options not imlemented yet in gophercloud
func (r *VolumeTypeAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	opts := &libclient.VolumeTypeListOpts{}
	volumeTypeList := []libclient.VolumeType{}
	err = ctx.client.List(&volumeTypeList, opts)
	if err != nil {
		return
	}
	for i := range volumeTypeList {
		volumeType := &VolumeType{volumeTypeList[i]}
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
		s := &libclient.VolumeType{}
		err = ctx.client.Get(s, volumeType.ID)
		if err != nil {
			if ctx.client.IsNotFound(err) {
				updater := func(tx *libmodel.Tx) (err error) {
					m := &model.VolumeType{
						Base: model.Base{ID: volumeType.ID},
					}
					return tx.Delete(m)
				}
				updates = append(updates, updater)
				err = nil
			} else {
				return
			}
		}
	}
	return
}

type NetworkAdapter struct {
}

func (r *NetworkAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	networkList := []libclient.Network{}
	opts := &libclient.NetworkListOpts{}
	err = ctx.client.List(&networkList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, network := range networkList {
		m := &model.Network{
			Base: model.Base{ID: network.ID},
		}
		n := &Network{network}
		n.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

func (r *NetworkAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	networkList := []libclient.Network{}
	opts := &libclient.NetworkListOpts{}
	err = ctx.client.List(&networkList, opts)
	if err != nil {
		return
	}
	for i := range networkList {
		network := &Network{networkList[i]}
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
		s := &libclient.Network{}
		err = ctx.client.Get(s, network.ID)
		if err != nil {
			if ctx.client.IsNotFound(err) {
				updater := func(tx *libmodel.Tx) (err error) {
					m := &model.Network{
						Base: model.Base{ID: network.ID},
					}
					return tx.Delete(m)
				}
				updates = append(updates, updater)
				err = nil
			} else {
				return
			}
		}
	}
	return
}

type SubnetAdapter struct {
}

func (r *SubnetAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	subnetList := []libclient.Subnet{}
	opts := &libclient.SubnetListOpts{}
	err = ctx.client.List(&subnetList, opts)
	if err != nil {
		return
	}
	list := fb.NewList()
	for _, subnet := range subnetList {
		m := &model.Subnet{
			Base: model.Base{ID: subnet.ID},
		}
		s := &Subnet{subnet}
		s.ApplyTo(m)
		list.Append(m)
	}
	itr = list.Iter()

	return
}

func (r *SubnetAdapter) GetUpdates(ctx *Context) (updates []Updater, err error) {
	subnetList := []libclient.Subnet{}
	opts := &libclient.SubnetListOpts{}
	err = ctx.client.List(&subnetList, opts)
	if err != nil {
		return
	}
	for i := range subnetList {
		subnet := &Subnet{subnetList[i]}
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
		s := &libclient.Subnet{}
		err = ctx.client.Get(s, subnet.ID)
		if err != nil {
			if ctx.client.IsNotFound(err) {
				updater := func(tx *libmodel.Tx) (err error) {
					m := &model.Subnet{
						Base: model.Base{ID: subnet.ID},
					}
					return tx.Delete(m)
				}
				updates = append(updates, updater)
				err = nil
			} else {
				return
			}
		}
	}
	return
}
