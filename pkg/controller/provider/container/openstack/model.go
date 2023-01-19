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
		&NetworkAdapter{},
	}
}

// Updates the DB based on
// changes described by an Event.
type Updater func(tx *libmodel.Tx) error

// Adapter context.
type Context struct {
	// Context.
	ctx context.Context
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
	// Apply an event to the inventory model.
	GetUpdates(ctx *Context, lastSync time.Time) (updates []Updater, err error)
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

func (r *RegionAdapter) GetUpdates(ctx *Context, lastSync time.Time) (updates []Updater, err error) {
	opts := &RegionListOpts{}
	regionList := []Region{}
	err = ctx.client.list(&regionList, opts)
	if err != nil {
		return
	}
	for _, region := range regionList {
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Region{
				Base: model.Base{ID: region.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, &NotFound{}) {
					region.ApplyTo(m)
					err = tx.Insert(m)
					return
				}
				return
			}
			region.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	// TODO: delete unexisting regions
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

func (r *ProjectAdapter) GetUpdates(ctx *Context, lastSync time.Time) (updates []Updater, err error) {
	opts := &ProjectListOpts{}
	projectList := []Project{}
	err = ctx.client.list(&projectList, opts)
	if err != nil {
		return
	}
	for _, project := range projectList {
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Project{
				Base: model.Base{ID: project.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, &NotFound{}) {
					project.ApplyTo(m)
					err = tx.Insert(m)
					return
				}
				return
			}
			project.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	// TODO: delete unexisting projects
	return
}

type ImageAdapter struct {
}

func (r *ImageAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &ImageListOpts{}
	imageList := []Image{}
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
	}
	itr = list.Iter()

	return
}

func (r *ImageAdapter) GetUpdates(ctx *Context, lastSync time.Time) (updates []Updater, err error) {
	opts := &ImageListOpts{}
	opts.setUpdateAtQueryFilterGTE(lastSync)
	imageList := []Image{}
	err = ctx.client.list(&imageList, opts)
	if err != nil {
		return
	}
	for _, image := range imageList {
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
					if errors.Is(err, &NotFound{}) {
						image.ApplyTo(m)
						err = tx.Insert(m)
						return
					}
					return
				}
				image.ApplyTo(m)
				err = tx.Update(m)
				return
			}
			updates = append(updates, updater)
		}
	}
	return
}

type FlavorAdapter struct {
}

func (r *FlavorAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &FlavorListOpts{}
	flavorList := []Flavor{}
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

	return
}

func (r *FlavorAdapter) GetUpdates(ctx *Context, lastSync time.Time) (updates []Updater, err error) {
	opts := &FlavorListOpts{}
	flavorList := []Flavor{}
	err = ctx.client.list(&flavorList, opts)
	if err != nil {
		return
	}
	for _, flavor := range flavorList {
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Flavor{
				Base: model.Base{ID: flavor.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, &NotFound{}) {
					flavor.ApplyTo(m)
					err = tx.Insert(m)
					return
				}
				return
			}
			flavor.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	// TODO: delete unexisting flavors
	return
}

// VM adapter.
type VMAdapter struct {
}

// List the collection.
func (r *VMAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	opts := &VMListOpts{}
	vmList := []VM{}
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

	return
}

// Get updates since last sync.
func (r *VMAdapter) GetUpdates(ctx *Context, lastSync time.Time) (updates []Updater, err error) {
	opts := &VMListOpts{}
	opts.ChangesSince = lastSync.Format(time.RFC3339)
	vmList := []VM{}
	err = ctx.client.list(&vmList, opts)
	if err != nil {
		return
	}
	for _, server := range vmList {
		switch server.Status {
		case VMStatusDeleted, VMStatusSoftDeleted:
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.VM{
					Base: model.Base{ID: server.ID},
				}
				server.ApplyTo(m)
				err = tx.Delete(m)
				return
			}
			updates = append(updates, updater)

		default:
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.VM{
					Base: model.Base{ID: server.ID},
				}
				err = tx.Get(m)
				if err != nil {
					if errors.Is(err, &NotFound{}) {
						server.ApplyTo(m)
						err = tx.Insert(m)
						return
					}
					return
				}
				server.ApplyTo(m)
				err = tx.Update(m)
				return
			}
			updates = append(updates, updater)
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

func (r *SnapshotAdapter) GetUpdates(ctx *Context, lastSync time.Time) (updates []Updater, err error) {
	snapshotList := []Snapshot{}
	err = ctx.client.list(&snapshotList, nil)
	if err != nil {
		return
	}
	for _, snapshot := range snapshotList {
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Snapshot{
				Base: model.Base{ID: snapshot.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, &NotFound{}) {
					snapshot.ApplyTo(m)
					err = tx.Insert(m)
					return
				}
				return
			}
			snapshot.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	// TODO: delete unexisting snapshots
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
func (r *VolumeAdapter) GetUpdates(ctx *Context, lastSync time.Time) (updates []Updater, err error) {
	opts := &VolumeListOpts{}
	volumeList := []Volume{}
	err = ctx.client.list(&volumeList, opts)
	if err != nil {
		return
	}
	for _, volume := range volumeList {
		switch volume.Status {
		case VolumeStatusDeleting:
			updater := func(tx *libmodel.Tx) (err error) {
				m := &model.Volume{
					Base: model.Base{ID: volume.ID},
				}
				volume.ApplyTo(m)
				err = tx.Delete(m)
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
					if errors.Is(err, &NotFound{}) {
						volume.ApplyTo(m)
						err = tx.Insert(m)
						return
					}
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

type NetworkAdapter struct {
}

func (r *NetworkAdapter) List(ctx *Context) (itr fb.Iterator, err error) {
	networkList := []Network{}
	err = ctx.client.list(&networkList, nil)
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

func (r *NetworkAdapter) GetUpdates(ctx *Context, lastSync time.Time) (updates []Updater, err error) {
	networkList := []Network{}
	err = ctx.client.list(&networkList, nil)
	if err != nil {
		return
	}
	for _, network := range networkList {
		updater := func(tx *libmodel.Tx) (err error) {
			m := &model.Network{
				Base: model.Base{ID: network.ID},
			}
			err = tx.Get(m)
			if err != nil {
				if errors.Is(err, &NotFound{}) {
					network.ApplyTo(m)
					err = tx.Insert(m)
					return
				}
				return
			}
			network.ApplyTo(m)
			err = tx.Update(m)
			return
		}
		updates = append(updates, updater)
	}
	// TODO: delete unexisting networks
	return
}
