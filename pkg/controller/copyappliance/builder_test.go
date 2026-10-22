package copyappliance

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	vspheremodel "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/settings"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// The bounds the builder's GenerateName prefixes have to stay inside: vCenter
// rejects a VM name over 80 characters, and the API server appends its own
// suffix to a GenerateName.
const (
	maxVMNameLength           = 80
	generatedNameSuffixLength = 5
)

// fakeInventory serves an inventory from in-memory maps. Only Find and Get are
// implemented; the embedded interface is nil, so a lookup through any other
// method of web.Client panics rather than quietly returning a zero value.
type fakeInventory struct {
	web.Client
	vm          model.VM
	template    model.VM
	folders     map[string]model.Folder
	datacenters map[string]model.Datacenter
	hosts       map[string]model.Host
	clusters    map[string]model.Cluster
	// There is deliberately no network or datastore map. Placement resolves
	// neither — the network and the datastore are the copy appliance template's spec
	// — and Get rejects the attempt, so a regression to taking either from the
	// inventory fails rather than passes.
}

// Find resolves the ref it is given, so a test can tell which of the two VMs a
// value was taken from.
func (r *fakeInventory) Find(resource interface{}, vmRef ref.Ref) error {
	out, ok := resource.(*model.VM)
	if !ok {
		return fmt.Errorf("unexpected Find for %T", resource)
	}
	switch vmRef.ID {
	case r.vm.ID:
		*out = r.vm
	case r.template.ID:
		*out = r.template
	default:
		return fmt.Errorf("vm %q not found", vmRef.ID)
	}
	return nil
}

func (r *fakeInventory) Get(resource interface{}, id string) error {
	switch out := resource.(type) {
	case *model.Folder:
		return lookup(r.folders, id, out, "folder")
	case *model.Datacenter:
		return lookup(r.datacenters, id, out, "datacenter")
	case *model.Host:
		return lookup(r.hosts, id, out, "host")
	case *model.Cluster:
		return lookup(r.clusters, id, out, "cluster")
	}
	return fmt.Errorf("unexpected Get for %T", resource)
}

func lookup[T any](from map[string]T, id string, out *T, kind string) error {
	found, ok := from[id]
	if !ok {
		return fmt.Errorf("%s %q not found", kind, id)
	}
	*out = found
	return nil
}

// testRef identifies the source VM. The fake ignores it: which VM is returned
// is decided by the fixture, not by the ref.
var testRef = ref.Ref{ID: "vm-101", Name: "web-01"}

const testMigrationUID = types.UID("6f1e2a3b-4c5d-6e7f-8091-a2b3c4d5e6f7")

// testInventory is a source VM and the provider's copy appliance template, each on a
// clustered host with one disk and in a folder of its own. Keeping the two
// apart is what makes it visible that the datacenter and the resource pool are
// the template's and only the attached disks are the source VM's. Every path
// is what PathBuilder would produce for that topology, hidden
// vm/host/network/datastore folders included.
func testInventory() *fakeInventory {
	resource := func(id, path string) model.Resource {
		return model.Resource{ID: id, Path: path}
	}
	vmResource := func(id, path, folder string) model.Resource {
		return model.Resource{
			ID:     id,
			Path:   path,
			Parent: vspheremodel.Ref{Kind: vspheremodel.FolderKind, ID: folder},
		}
	}
	return &fakeInventory{
		vm: model.VM{
			VM1: model.VM1{
				VM0:  vmResource("vm-101", "/DC0/vm/apps/web-01", "folder-apps"),
				Host: "host-1",
				Disks: []vspheremodel.Disk{
					{
						Key:       2000,
						File:      "[datastore1] web-01/disk-0.vmdk",
						Serial:    "6000C297-7d53-fad7-e8b4-5194193802f7",
						Capacity:  16 << 30,
						Datastore: vspheremodel.Ref{Kind: vspheremodel.DsKind, ID: "ds-1"},
					},
				},
			},
			// The source VM's own NIC. Nothing reads it; it is here so that
			// a build taking its network from the source VM would be caught.
			NICs: []vspheremodel.NIC{
				{Network: vspheremodel.Ref{Kind: vspheremodel.NetKind, ID: "net-1"}, Index: 0},
			},
		},
		template: model.VM{
			VM1: model.VM1{
				VM0:  vmResource("vm-900", "/DC0/vm/templates/vcenter-copy-appliance-template", "folder-templates"),
				Host: "host-1",
				Disks: []vspheremodel.Disk{
					{
						Key:       2000,
						File:      "[templates] vcenter-copy-appliance-template/disk-0.vmdk",
						Capacity:  8 << 30,
						Datastore: vspheremodel.Ref{Kind: vspheremodel.DsKind, ID: "ds-2"},
					},
				},
			},
		},
		folders: map[string]model.Folder{
			"folder-apps": {
				Resource:   resource("folder-apps", "/DC0/vm/apps"),
				Folder:     "folder-vm",
				Datacenter: "dc-1",
			},
			"folder-templates": {
				Resource:   resource("folder-templates", "/DC0/vm/templates"),
				Folder:     "folder-vm",
				Datacenter: "dc-1",
			},
			"folder-vm": {
				Resource:   resource("folder-vm", "/DC0/vm"),
				Datacenter: "dc-1",
			},
		},
		datacenters: map[string]model.Datacenter{
			"dc-1": {Resource: resource("dc-1", "/DC0")},
		},
		hosts: map[string]model.Host{
			"host-1": {
				Resource: resource("host-1", "/DC0/host/Cluster0/esx1.example.com"),
				Cluster:  "cluster-1",
			},
		},
		clusters: map[string]model.Cluster{
			"cluster-1": {Resource: resource("cluster-1", "/DC0/host/Cluster0")},
		},
	}
}

// vmParent sets the source VM's parent.
func (r *fakeInventory) vmParent(kind, id string) *fakeInventory {
	r.vm.Parent = vspheremodel.Ref{Kind: kind, ID: id}
	return r
}

// templateParent sets the template's parent, which is how the appliance's
// folder is found.
func (r *fakeInventory) templateParent(kind, id string) *fakeInventory {
	r.template.Parent = vspheremodel.Ref{Kind: kind, ID: id}
	return r
}

func TestPlacementFollowsTheTemplate(t *testing.T) {
	inventory := testInventory()
	spec := api.CopyApplianceSpec{}
	if err := (&Builder{Provider: testProvider(), Inventory: inventory}).placement(testCopyApplianceTemplate(), &spec); err != nil {
		t.Fatalf("placement: %v", err)
	}
	tests := []struct {
		field string
		got   string
		want  string
	}{
		{"Template", spec.Template, "/DC0/vm/templates/vcenter-copy-appliance-template"},
		{"Folder", spec.Folder, "/DC0/vm/templates"},
		{"Datacenter", spec.Datacenter, "/DC0"},
		// The appliance is not placed on the template's host; the host is
		// only how its compute resource, and so the pool below, is found.
		{"ResourcePool", spec.ResourcePool, "/DC0/host/Cluster0/Resources"},
		// The appliance's own home, not where the disks it serves live.
		{"Datastore", spec.Datastore, "templates"},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.field, tc.got, tc.want)
		}
	}
	if len(spec.AttachDisks) != 0 {
		t.Errorf("AttachDisks = %+v, want placement to leave them alone", spec.AttachDisks)
	}
}

// Where the appliance is cloned from and to is the copy appliance template's spec,
// not where the inventory has the template. The two agree unless the template
// was moved or renamed after the import, and the spec is what the import was
// told to do.
func TestPlacementFollowsTheCopyApplianceTemplateSpec(t *testing.T) {
	inventory := testInventory()
	copyApplianceTemplate := testCopyApplianceTemplate()
	copyApplianceTemplate.Spec.Folder = "/DC0/vm/somewhere-else"
	copyApplianceTemplate.Spec.TemplateName = "renamed-since-import"
	copyApplianceTemplate.Spec.Datastore = "another-datastore"

	spec := api.CopyApplianceSpec{}
	if err := (&Builder{Provider: testProvider(), Inventory: inventory}).placement(copyApplianceTemplate, &spec); err != nil {
		t.Fatalf("placement: %v", err)
	}
	if spec.Folder != "/DC0/vm/somewhere-else" {
		t.Errorf("Folder = %q, want the spec's folder", spec.Folder)
	}
	if spec.Template != "/DC0/vm/somewhere-else/renamed-since-import" {
		t.Errorf("Template = %q, want the spec's folder and template name", spec.Template)
	}
	if spec.Datastore != "another-datastore" {
		t.Errorf("Datastore = %q, want the spec's datastore", spec.Datastore)
	}
}

// A folder path need not be absolute: the finder that resolves it has the
// datacenter set from the placement below, and joining a template name onto it
// must not make it absolute.
func TestPlacementRelativeFolder(t *testing.T) {
	copyApplianceTemplate := testCopyApplianceTemplate()
	copyApplianceTemplate.Spec.Folder = "vm/templates"

	spec := api.CopyApplianceSpec{}
	if err := (&Builder{Provider: testProvider(), Inventory: testInventory()}).placement(copyApplianceTemplate, &spec); err != nil {
		t.Fatalf("placement: %v", err)
	}
	if spec.Template != "vm/templates/vcenter-copy-appliance-template" {
		t.Errorf("Template = %q, want it relative like the folder it is in", spec.Template)
	}
}

// Nested folders carry Datacenter from the inventory (resolved at serve
// time), so placement does not walk the folder chain itself.
func TestPlacementNestedFolder(t *testing.T) {
	inventory := testInventory().templateParent(vspheremodel.FolderKind, "folder-team")
	inventory.folders["folder-team"] = model.Folder{
		Resource:   model.Resource{ID: "folder-team", Path: "/DC0/vm/templates/team"},
		Folder:     "folder-templates",
		Datacenter: "dc-1",
	}
	spec := api.CopyApplianceSpec{}
	if err := (&Builder{Provider: testProvider(), Inventory: inventory}).placement(testCopyApplianceTemplate(), &spec); err != nil {
		t.Fatalf("placement: %v", err)
	}
	if spec.Datacenter != "/DC0" {
		t.Errorf("Datacenter = %q, want /DC0", spec.Datacenter)
	}
}

// A datacenter can itself sit in a folder, which is why the datacenter is
// resolved rather than read off the first segment of a path.
func TestPlacementDatacenterInAFolder(t *testing.T) {
	inventory := testInventory()
	inventory.datacenters["dc-1"] = model.Datacenter{
		Resource: model.Resource{ID: "dc-1", Path: "/east/DC0"},
	}
	spec := api.CopyApplianceSpec{}
	if err := (&Builder{Provider: testProvider(), Inventory: inventory}).placement(testCopyApplianceTemplate(), &spec); err != nil {
		t.Fatalf("placement: %v", err)
	}
	if spec.Datacenter != "/east/DC0" {
		t.Errorf("Datacenter = %q, want /east/DC0", spec.Datacenter)
	}
}

// A host outside a cluster is collected as a Cluster with the ComputeResource
// variant, so it has a root resource pool like any other.
func TestPlacementStandaloneHost(t *testing.T) {
	inventory := testInventory()
	inventory.hosts["host-1"] = model.Host{
		Resource: model.Resource{ID: "host-1", Path: "/DC0/host/esx1.example.com/esx1.example.com"},
		Cluster:  "cr-1",
	}
	inventory.clusters["cr-1"] = model.Cluster{
		Resource: model.Resource{
			ID:      "cr-1",
			Variant: vspheremodel.ComputeResource,
			Path:    "/DC0/host/esx1.example.com",
		},
	}
	spec := api.CopyApplianceSpec{}
	if err := (&Builder{Provider: testProvider(), Inventory: inventory}).placement(testCopyApplianceTemplate(), &spec); err != nil {
		t.Fatalf("placement: %v", err)
	}
	if spec.ResourcePool != "/DC0/host/esx1.example.com/Resources" {
		t.Errorf("ResourcePool = %q, want the compute resource's root pool", spec.ResourcePool)
	}
}

// Every placement input is the template's, so every way placement can fail is
// a way the template can be wrong.
func TestPlacementErrors(t *testing.T) {
	tests := []struct {
		name                  string
		setup                 func(*fakeInventory)
		copyApplianceTemplate func(*api.CopyApplianceTemplate)
		want                  string
	}{
		{
			name:                  "template not imported yet",
			copyApplianceTemplate: func(x *api.CopyApplianceTemplate) { x.Status.Template.Moref = "" },
			want:                  "moref",
		},
		{
			name:  "template not in the inventory",
			setup: func(i *fakeInventory) { i.template = model.VM{} },
			want:  "not found",
		},
		{
			name:  "template in a vApp rather than a folder",
			setup: func(i *fakeInventory) { i.templateParent("VirtualApp", "vapp-1") },
			want:  "not in an inventory folder",
		},
		{
			name:  "no host",
			setup: func(i *fakeInventory) { i.template.Host = "" },
			want:  "not found",
		},
		{
			name: "folder with no datacenter",
			setup: func(i *fakeInventory) {
				i.folders["folder-templates"] = model.Folder{
					Resource: model.Resource{ID: "folder-templates", Path: "/DC0/vm/templates"},
				}
			},
			want: "not under a datacenter",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inventory := testInventory()
			copyApplianceTemplate := testCopyApplianceTemplate()
			if tc.setup != nil {
				tc.setup(inventory)
			}
			if tc.copyApplianceTemplate != nil {
				tc.copyApplianceTemplate(copyApplianceTemplate)
			}
			spec := api.CopyApplianceSpec{}
			err := (&Builder{Provider: testProvider(), Inventory: inventory}).placement(copyApplianceTemplate, &spec)
			if err == nil {
				t.Fatalf("placement succeeded, want an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("placement error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

// The appliance takes nothing but disks from the VM it serves, so a source VM
// with no host and no folder of its own still places.
func TestAttachDisksIsAllTheSourceVMContributes(t *testing.T) {
	withSettings(t, testSettings())
	inventory := testInventory().vmParent("VirtualApp", "vapp-1")
	inventory.vm.Host = ""

	appliance, err := (&Builder{Provider: testProvider(), Inventory: inventory}).Appliance(testCopyApplianceTemplate(), testRef, testMigrationUID)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if appliance.Spec.Folder != "/DC0/vm/templates" {
		t.Errorf("Folder = %q, want the template's", appliance.Spec.Folder)
	}
	if len(appliance.Spec.AttachDisks) != 1 {
		t.Fatalf("AttachDisks = %+v, want the source VM's one disk", appliance.Spec.AttachDisks)
	}
	if appliance.Spec.AttachDisks[0].VMDKPath != "[datastore1] web-01/disk-0.vmdk" {
		t.Errorf("AttachDisks[0].VMDKPath = %q, want the source VM's",
			appliance.Spec.AttachDisks[0].VMDKPath)
	}
}

func TestBuildRejectsAnUnknownSourceVM(t *testing.T) {
	withSettings(t, testSettings())
	inventory := testInventory()

	_, err := (&Builder{Provider: testProvider(), Inventory: inventory}).Appliance(testCopyApplianceTemplate(), ref.Ref{ID: "vm-404"}, testMigrationUID)
	if err == nil {
		t.Fatal("build succeeded for a VM that is not in the inventory")
	}
	if !strings.Contains(err.Error(), "vm-404") {
		t.Errorf("error = %q, want it to name the missing VM", err)
	}
}

// testCopyApplianceTemplate is the provider's copy appliance template, imported and placed. The
// spec is where the import put it; the moref resolves to that same template VM
// in the fixture inventory.
func testCopyApplianceTemplate() *api.CopyApplianceTemplate {
	return &api.CopyApplianceTemplate{
		ObjectMeta: meta.ObjectMeta{Namespace: "forklift", Name: "vcenter-copy-appliance-template"},
		Spec: api.CopyApplianceTemplateSpec{
			TemplateName: "vcenter-copy-appliance-template",
			Folder:       "/DC0/vm/templates",
			Datastore:    "templates",
		},
		Status: api.CopyApplianceTemplateStatus{
			Phase:    api.CopyApplianceTemplatePhaseSucceeded,
			Template: api.TemplateStatus{Moref: "vm-900"},
		},
	}
}

func testProvider() *api.Provider {
	vsphere := api.VSphere
	return &api.Provider{
		ObjectMeta: meta.ObjectMeta{
			Name:      "vcenter",
			Namespace: "forklift",
			UID:       types.UID("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"),
		},
		Spec: api.ProviderSpec{Type: &vsphere},
		Status: api.ProviderStatus{
			CopyApplianceSSHPrivateSecret: "copy-appliance-ssh-keys-vcenter-private",
			CopyApplianceSSHPublicSecret:  "copy-appliance-ssh-keys-vcenter-public",
		},
	}
}

// withSettings installs appliance settings for the duration of a test.
func withSettings(t *testing.T, applied settings.CopyAppliance) {
	t.Helper()
	previous := Settings.CopyAppliance
	Settings.CopyAppliance = applied
	t.Cleanup(func() { Settings.CopyAppliance = previous })
}

func testSettings() settings.CopyAppliance {
	return settings.CopyAppliance{
		SSHUser:        "root",
		ContainerImage: "quay.io/kubev2v/nbd-container:latest",
	}
}

func TestBuild(t *testing.T) {
	withSettings(t, testSettings())
	inventory := testInventory()
	provider := testProvider()

	appliance, err := (&Builder{Provider: provider, Inventory: inventory}).Appliance(testCopyApplianceTemplate(), testRef, testMigrationUID)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if appliance.Namespace != "forklift" {
		t.Errorf("Namespace = %q, want the provider's", appliance.Namespace)
	}
	// The API server names it; the appliance VM is cloned under that name.
	if appliance.Name != "" {
		t.Errorf("Name = %q, want it left for the API server", appliance.Name)
	}
	if appliance.GenerateName == "" {
		t.Error("GenerateName is empty, want a prefix for the API server")
	}
	if len(appliance.GenerateName)+generatedNameSuffixLength > maxVMNameLength {
		t.Errorf("GenerateName %q generates a name vCenter would refuse", appliance.GenerateName)
	}

	// The labels are the appliance's identity: this is the set every lookup
	// selects on, so it has to be exactly what the Labeler produces.
	labeler := Labeler{}
	wantLabels := labeler.ApplianceLabels(provider, testMigrationUID, testRef.ID)
	if !reflect.DeepEqual(appliance.Labels, wantLabels) {
		t.Errorf("Labels = %v, want %v", appliance.Labels, wantLabels)
	}

	spec := appliance.Spec
	if spec.Provider.Namespace != "forklift" || spec.Provider.Name != "vcenter" {
		t.Errorf("Provider = %v, want forklift/vcenter", spec.Provider)
	}
	if spec.ContainerImage != testSettings().ContainerImage {
		t.Errorf("ContainerImage = %q, want it from settings", spec.ContainerImage)
	}
	// The copy appliance template build injects the matching public key; the private
	// half and the certificates live with the provider.
	if spec.Secret.Namespace != "forklift" || spec.Secret.Name != "copy-appliance-ssh-keys-vcenter-private" {
		t.Errorf("Secret = %v, want the copyApplianceTemplate private secret in the provider namespace", spec.Secret)
	}
	if len(spec.AttachDisks) != 1 {
		t.Fatalf("AttachDisks = %+v, want one disk from inventory", spec.AttachDisks)
	}
	if spec.AttachDisks[0].VMDKPath != "[datastore1] web-01/disk-0.vmdk" {
		t.Errorf("AttachDisks[0].VMDKPath = %q", spec.AttachDisks[0].VMDKPath)
	}
	if spec.AttachDisks[0].Serial != "6000C297-7d53-fad7-e8b4-5194193802f7" {
		t.Errorf("AttachDisks[0].Serial = %q", spec.AttachDisks[0].Serial)
	}
	// Placement is the template's; only the disks above are the source VM's.
	if spec.Template != "/DC0/vm/templates/vcenter-copy-appliance-template" {
		t.Errorf("Template = %q, want the copy appliance template's inventory path", spec.Template)
	}
	if spec.Folder != "/DC0/vm/templates" || spec.Datastore != "templates" {
		t.Errorf("placement = (%q, %q), want the template's", spec.Folder, spec.Datastore)
	}
}

// Without an image the appliance is cloned, configured, and then has nothing
// to serve exports with.
func TestBuildRejectsAnUnconfiguredContainerImage(t *testing.T) {
	applied := testSettings()
	applied.ContainerImage = ""
	withSettings(t, applied)
	inventory := testInventory()

	_, err := (&Builder{Provider: testProvider(), Inventory: inventory}).Appliance(testCopyApplianceTemplate(), testRef, testMigrationUID)
	if err == nil {
		t.Fatal("build succeeded without a container image")
	}
	if !strings.Contains(err.Error(), settings.CopyApplianceContainerImage) {
		t.Errorf("error = %q, want it to name %s", err, settings.CopyApplianceContainerImage)
	}
}

// The setup pod reads the image with no credential and no cluster-local
// resolution, so a reference naming no registry would be pulled from Docker
// Hub. Rejected here, it is reported on the provider rather than as a pod that
// failed part way through a deploy.
func TestBuildRejectsAnImageThatIsNotAPullSpec(t *testing.T) {
	for _, image := range []string{"nbd-container:latest", "kubev2v/nbd-container:latest"} {
		t.Run(image, func(t *testing.T) {
			applied := testSettings()
			applied.ContainerImage = image
			withSettings(t, applied)

			_, err := (&Builder{Provider: testProvider(), Inventory: testInventory()}).
				Appliance(testCopyApplianceTemplate(), testRef, testMigrationUID)

			if err == nil {
				t.Fatalf("build succeeded with %q as the container image", image)
			}
			if !strings.Contains(err.Error(), settings.CopyApplianceContainerImage) {
				t.Errorf("error = %q, want it to name %s", err, settings.CopyApplianceContainerImage)
			}
		})
	}
}

func TestBuildRejectsANonVSphereProvider(t *testing.T) {
	withSettings(t, testSettings())
	inventory := testInventory()
	provider := testProvider()
	ovirt := api.OVirt
	provider.Spec.Type = &ovirt

	_, err := (&Builder{Provider: provider, Inventory: inventory}).Appliance(testCopyApplianceTemplate(), testRef, testMigrationUID)
	if err == nil {
		t.Fatal("build succeeded for an oVirt provider")
	}
	if !strings.Contains(err.Error(), "only supported for vSphere") {
		t.Errorf("error = %q, want it to say vSphere only", err)
	}
}
