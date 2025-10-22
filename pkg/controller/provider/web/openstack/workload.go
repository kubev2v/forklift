package openstack

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/openstack"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	WorkloadCollection = "workloads"
	WorkloadsRoot      = ProviderRoot + "/" + WorkloadCollection
	WorkloadRoot       = WorkloadsRoot + "/:" + VMParam
)

// Virtual Machine handler.
type WorkloadHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *WorkloadHandler) AddRoutes(e *gin.Engine) {
	e.GET(WorkloadRoot, h.Get)
}

// List resources in a REST collection.
func (h WorkloadHandler) List(ctx *gin.Context) {
}

// Get a specific REST resource.
func (h WorkloadHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.VM{
		Base: model.Base{
			ID: ctx.Param(VMParam),
		},
	}
	db := h.Collector.DB()
	err = db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	defer func() {
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
		}
	}()
	if err != nil {
		return
	}
	h.Detail = model.MaxDetail
	r := Workload{}
	r.VM.With(m)
	err = r.Expand(h.Collector.DB())
	if err != nil {
		return
	}
	r.Link(h.Provider)

	ctx.JSON(http.StatusOK, r)
}

// Workload
type Workload struct {
	SelfLink string `json:"selfLink"`
	XVM
}

// Build self link (URI).
func (r *Workload) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		WorkloadRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
	r.XVM.Link(p)
}

// Expanded: VM.
type XVM struct {
	VM
	Image       Image        `json:"image"`
	Flavor      Flavor       `json:"flavor"`
	Networks    []Network    `json:"networks"`
	Subnets     []Subnet     `json:"subnets"`
	Volumes     []Volume     `json:"volumes,omitempty"`
	VolumeTypes []VolumeType `json:"volumeTypes,omitempty"`
	Snapshots   []Snapshot   `json:"snapshots,omitempty"`
}

// Expand references.
func (r *XVM) Expand(db libmodel.DB) (err error) {
	var imageID string
	if r.ImageID != "" {
		imageID = r.ImageID
	}
	flavor := model.Flavor{Base: model.Base{
		ID: r.FlavorID,
	}}
	err = db.Get(&flavor)
	if err != nil {
		return
	}
	r.Flavor.With(&flavor)

	var networks []Network
	var subnets []Subnet

	for name := range r.Addresses {
		networkList := []model.Network{}
		err = db.List(&networkList, model.ListOptions{
			Predicate: libmodel.Eq("Name", name),
			Detail:    model.MaxDetail,
		})
		if err != nil {
			return
		}
		for _, networkModel := range networkList {
			network := &Network{}
			network.With(&networkModel)
			networks = append(networks, *network)
			for _, subnetID := range network.Subnets {
				subnetModel := model.Subnet{Base: model.Base{
					ID: subnetID,
				}}
				err = db.Get(&subnetModel)
				if err != nil {
					return
				}
				subnet := &Subnet{}
				subnet.With(&subnetModel)
				subnets = append(subnets, *subnet)
			}
		}
	}
	r.Networks = networks
	r.Subnets = subnets

	var volumes []Volume
	var volumeTypes []VolumeType
	var snapshots []Snapshot

	volumeTypeCache := map[string]VolumeType{}

	for _, attachedVolume := range r.AttachedVolumes {
		volume := &Volume{}
		volumeModel := model.Volume{
			Base: model.Base{ID: attachedVolume.ID},
		}
		err = db.Get(&volumeModel)
		if err != nil {
			return
		}
		volume.With(&volumeModel)
		volumes = append(volumes, *volume)

		if _, ok := volumeTypeCache[volume.VolumeType]; !ok {
			volumeTypes := []model.VolumeType{}
			err = db.List(&volumeTypes, model.ListOptions{
				Predicate: libmodel.Eq("Name", volume.VolumeType),
				Detail:    model.MaxDetail,
			})
			if err != nil {
				return
			}
			volumeType := &VolumeType{}
			volumeType.With(&volumeTypes[0])
			volumeTypeCache[volume.VolumeType] = *volumeType
		}

		volumeType := volumeTypeCache[volume.VolumeType]
		found := false
		for i := range volumeTypes {
			if volumeTypes[i].ID == volumeType.ID {
				found = true
			}
		}
		if !found {
			volumeTypes = append(volumeTypes, volumeType)
		}

		snapshotsList := []model.Snapshot{}
		err = db.List(&snapshotsList, model.ListOptions{
			Predicate: libmodel.Eq("volumeID", attachedVolume.ID),
			Detail:    model.MaxDetail,
		})
		if err != nil {
			return
		}
		for _, snapshotModel := range snapshotsList {
			snapshot := &Snapshot{}
			snapshot.With(&snapshotModel)
			snapshots = append(snapshots, *snapshot)
		}

		if imageID == "" && volume.Bootable == "true" {
			if volumeImageID, ok := volume.VolumeImageMetadata["image_id"]; ok {
				imageID = volumeImageID
			}
		}

	}

	r.Volumes = volumes
	r.VolumeTypes = volumeTypes
	r.Snapshots = snapshots

	if imageID != "" {
		image := model.Image{Base: model.Base{
			ID: imageID,
		}}

		err = db.Get(&image)
		if err != nil {
			// The image the VM has been based on could have been removed
			if errors.Is(err, model.NotFound) {
				err = nil
				return
			}
			return
		}
		r.Image.With(&image)
	}

	return
}

// Build self link (URI).
func (r *XVM) Link(p *api.Provider) {
	r.VM.Link(p)
	r.Image.Link(p)
	r.Flavor.Link(p)
	for i := range r.Networks {
		network := &r.Networks[i]
		network.Link(p)
	}
	for i := range r.Subnets {
		subnet := &r.Subnets[i]
		subnet.Link(p)
	}
	for i := range r.VolumeTypes {
		volumeType := &r.VolumeTypes[i]
		volumeType.Link(p)
	}
	for i := range r.Volumes {
		volume := &r.Volumes[i]
		volume.Link(p)
	}
	for i := range r.Snapshots {
		snapshot := &r.Snapshots[i]
		snapshot.Link(p)
	}
}

// Expand the workload.
func (r *Workload) Expand(db libmodel.DB) (err error) {
	// VM
	err = r.XVM.Expand(db)
	if err != nil {
		return err
	}

	return
}
