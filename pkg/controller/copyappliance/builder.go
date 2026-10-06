package copyappliance

import (
	"fmt"
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	vspheremodel "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Builder builds an uncreated CopyAppliance from aPowerOn:  !encrypted toehold template.
type Builder struct {
	Provider  *api.Provider
	Inventory web.Client
	Labeler   Labeler
}

func NewBuilder(provider *api.Provider) (*Builder, error) {
	inventory, err := web.NewClient(provider)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return &Builder{Provider: provider, Inventory: inventory}, nil
}

// Appliance builds a CopyAppliance that attaches the source VM's disks.
func (r *Builder) Appliance(toehold *api.ToeholdTemplate, vmRef ref.Ref, migrationUID types.UID) (*api.CopyAppliance, error) {
	appliance, err := r.build(toehold)
	if err != nil {
		return nil, err
	}
	if err = r.attachDisks(vmRef, &appliance.Spec); err != nil {
		return nil, err
	}
	appliance.GenerateName = appliancePrefix + vmRef.ID + "-"
	appliance.Labels = r.Labeler.ApplianceLabels(r.Provider, migrationUID, vmRef.ID)
	return appliance, nil
}

// Check builds a CopyAppliance with no attached disks (provider readiness probe).
func (r *Builder) Check(toehold *api.ToeholdTemplate) (*api.CopyAppliance, error) {
	appliance, err := r.build(toehold)
	if err != nil {
		return nil, err
	}
	appliance.GenerateName = checkPrefix
	appliance.Labels = r.Labeler.CheckLabels(r.Provider)
	return appliance, nil
}

func (r *Builder) build(toehold *api.ToeholdTemplate) (*api.CopyAppliance, error) {
	provider := r.Provider
	if provider.Type() != api.VSphere {
		return nil, liberr.New(fmt.Sprintf(
			"copy appliances are only supported for vSphere providers; %s is %s",
			provider.Name, provider.Type()))
	}
	if Settings.ContainerImage == "" {
		return nil, liberr.New(
			"the copy appliance container image is not configured; set " +
				settings.CopyApplianceContainerImage)
	}
	// The setup pod reads the image with no credential and no cluster-local
	// resolution, so rejecting it here reports the misconfiguration on the
	// provider rather than as a pod that failed part way through a deploy.
	if !isPullSpec(Settings.ContainerImage) {
		return nil, liberr.New(
			"the copy appliance container image must be a fully qualified pull spec; set "+
				settings.CopyApplianceContainerImage,
			"image", Settings.ContainerImage)
	}
	if provider.Status.ToeholdSSHPrivateSecret == "" {
		return nil, liberr.New(
			"provider has no toehold SSH private secret yet",
			"provider", provider.Name)
	}

	spec := api.CopyApplianceSpec{
		Provider: core.ObjectReference{
			Namespace: provider.Namespace,
			Name:      provider.Name,
		},
		Secret: core.ObjectReference{
			Namespace: provider.Namespace,
			Name:      provider.Status.ToeholdSSHPrivateSecret,
		},
		ContainerImage: Settings.ContainerImage,
	}
	if err := r.placement(toehold, &spec); err != nil {
		return nil, err
	}
	return &api.CopyAppliance{
		ObjectMeta: meta.ObjectMeta{Namespace: provider.Namespace},
		Spec:       spec,
	}, nil
}

// placement copies folder/datastore/template from the toehold and resolves
// datacenter + resource pool from inventory (by template moref; templates are
// not listed by path).
func (r *Builder) placement(toehold *api.ToeholdTemplate, spec *api.CopyApplianceSpec) error {
	spec.Folder = toehold.Spec.Folder
	spec.Datastore = toehold.Spec.Datastore
	spec.Template = path.Join(toehold.Spec.Folder, toehold.Spec.TemplateName)

	moRef := toehold.Status.Template.Moref
	if moRef == "" {
		return liberr.New(fmt.Sprintf(
			"toehold template %s has no moref to place the appliance from",
			toehold.Name))
	}
	vm := &model.VM{}
	if err := r.Inventory.Find(vm, ref.Ref{ID: moRef}); err != nil {
		return liberr.Wrap(err, "template", moRef)
	}
	if vm.Parent.Kind != vspheremodel.FolderKind {
		return liberr.New(fmt.Sprintf(
			"VM %s is not in an inventory folder; its parent is a %s",
			vm.ID, vm.Parent.Kind))
	}

	folder := &model.Folder{}
	if err := r.Inventory.Get(folder, vm.Parent.ID); err != nil {
		return err
	}
	if folder.Datacenter == "" {
		return liberr.New(fmt.Sprintf("folder %s is not under a datacenter", folder.ID))
	}
	datacenter := &model.Datacenter{}
	if err := r.Inventory.Get(datacenter, folder.Datacenter); err != nil {
		return err
	}
	spec.Datacenter = datacenter.Path

	host := &model.Host{}
	if err := r.Inventory.Get(host, vm.Host); err != nil {
		return err
	}
	cluster := &model.Cluster{}
	if err := r.Inventory.Get(cluster, host.Cluster); err != nil {
		return err
	}
	if pool := r.Provider.Setting(api.CopyApplianceResourcePool); pool != "" {
		spec.ResourcePool = pool
	} else {
		spec.ResourcePool = cluster.Path + "/" + rootResourcePool
	}
	return nil
}

func (r *Builder) attachDisks(vmRef ref.Ref, spec *api.CopyApplianceSpec) error {
	vm := &model.VM{}
	if err := r.Inventory.Find(vm, vmRef); err != nil {
		return liberr.Wrap(err, "vm", vmRef.String())
	}
	for _, disk := range vm.Disks {
		if disk.Shared || disk.RDM || disk.File == "" {
			continue
		}
		spec.AttachDisks = append(spec.AttachDisks, api.AttachedDisk{
			VMDKPath: disk.File,
			DiskKey:  disk.Key,
			Serial:   disk.Serial,
			Capacity: disk.Capacity,
		})
	}
	return nil
}
