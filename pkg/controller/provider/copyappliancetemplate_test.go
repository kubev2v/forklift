package provider

import (
	"context"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// withSyncSettings adds the template settings the sync writes. The placement
// comes from the provider; these come from the ForkliftController.
func withSyncSettings(t *testing.T) {
	t.Helper()
	withToeholdSettings(t)
	Settings.TemplateCPU = 4
	Settings.TemplateMemoryMiB = 8192
	Settings.BuilderImage = "builder:latest"
}

// syncProvider is a provider that has connected and built its inventory, which
// is what the sync gates on.
func syncProvider() *api.Provider {
	provider := checkProvider()
	provider.Status.SetCondition(
		libcnd.Condition{Type: ConnectionTestSucceeded, Status: True, Category: Required},
		libcnd.Condition{Type: InventoryCreated, Status: True, Category: Required})
	return provider
}

// syncTemplate is a copy appliance template as someone else created it: the fields the
// provider dictates are empty or wrong, and the ones it does not are set.
func syncTemplate() *api.CopyApplianceTemplate {
	return &api.CopyApplianceTemplate{
		ObjectMeta: meta.ObjectMeta{Namespace: "forklift", Name: "vcenter-toehold"},
		Spec: api.CopyApplianceTemplateSpec{
			Datastore:       "the-old-datastore",
			TransferNetwork: &core.ObjectReference{Name: "migration-net", Namespace: "openshift-mtv"},
			NodeSelector:    map[string]string{"kubernetes.io/arch": "amd64"},
		},
	}
}

func testToeholdSync(t *testing.T, provider *api.Provider, objs ...client.Object) *toeholdSync {
	t.Helper()
	cl := fake.NewClientBuilder().
		WithScheme(testCheckScheme(t)).
		WithObjects(objs...).
		WithStatusSubresource(&api.Provider{}, &api.CopyAppliance{}, &api.CopyApplianceTemplate{}).
		Build()
	return newToeholdSync(cl, provider)
}

func getTemplate(t *testing.T, s *toeholdSync) *api.CopyApplianceTemplate {
	t.Helper()
	found := &api.CopyApplianceTemplate{}
	key := client.ObjectKey{Namespace: "forklift", Name: "vcenter-toehold"}
	if err := s.client.Get(context.TODO(), key, found); err != nil {
		t.Fatalf("get template: %v", err)
	}
	return found
}

// The spec has fields the provider dictates and fields it does not. Replacing
// the whole spec would wipe the ones nothing else ever writes back.
func TestToeholdSyncPreservesFieldsTheProviderDoesNotOwn(t *testing.T) {
	withSyncSettings(t)
	provider := syncProvider()
	s := testToeholdSync(t, provider, provider, syncTemplate())

	if err := s.Run(context.TODO()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	spec := getTemplate(t, s).Spec
	if spec.TransferNetwork == nil || spec.TransferNetwork.Name != "migration-net" {
		t.Errorf("TransferNetwork = %v, want it left alone", spec.TransferNetwork)
	}
	if spec.NodeSelector["kubernetes.io/arch"] != "amd64" {
		t.Errorf("NodeSelector = %v, want it left alone", spec.NodeSelector)
	}

	// The fields the provider does own.
	if spec.Provider.Name != "vcenter" || spec.Provider.Namespace != "forklift" {
		t.Errorf("Provider = %v, want the provider", spec.Provider)
	}
	if spec.TemplateName != "vcenter-toehold" {
		t.Errorf("TemplateName = %q", spec.TemplateName)
	}
	if spec.BaseDisk.ContainerImage != "registry.example/rhel:9" {
		t.Errorf("BaseDisk = %+v", spec.BaseDisk)
	}
	if spec.Resources.CPU != 4 || spec.Resources.MemoryMiB != 8192 {
		t.Errorf("Resources = %+v", spec.Resources)
	}
	if spec.Datastore != "ds1" || spec.Folder != "/DC0/vm" || spec.Network != "VM Network" {
		t.Errorf("placement = (%q, %q, %q), want the provider's settings",
			spec.Datastore, spec.Folder, spec.Network)
	}
	if spec.BuilderImage != "builder:latest" {
		t.Errorf("BuilderImage = %q", spec.BuilderImage)
	}
}

// The template is created by the console or the API. A provider that makes its
// own would start a build nobody asked for.
func TestToeholdSyncDoesNotCreate(t *testing.T) {
	withSyncSettings(t)
	provider := syncProvider()
	s := testToeholdSync(t, provider, provider)

	if err := s.Run(context.TODO()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	list := &api.CopyApplianceTemplateList{}
	if err := s.client.List(context.TODO(), list); err != nil {
		t.Fatalf("list templates: %v", err)
	}
	if len(list.Items) != 0 {
		t.Errorf("created %d templates, want none", len(list.Items))
	}
}

// The provider reconciles every 30s. A sync that wrote every time would be a
// write per provider per 30s, and would wake the toehold controller each time.
func TestToeholdSyncIsIdempotent(t *testing.T) {
	withSyncSettings(t)
	provider := syncProvider()
	s := testToeholdSync(t, provider, provider, syncTemplate())

	if err := s.Run(context.TODO()); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	settled := getTemplate(t, s).ResourceVersion

	if err := s.Run(context.TODO()); err != nil {
		t.Fatalf("second pass: %v", err)
	}

	if got := getTemplate(t, s).ResourceVersion; got != settled {
		t.Errorf("the second pass wrote the template (%s -> %s), want no write", settled, got)
	}
}

// Ownership is what makes deleting the provider delete its template.
func TestToeholdSyncSetsOwnership(t *testing.T) {
	withSyncSettings(t)
	provider := syncProvider()

	t.Run("on a template it has not seen", func(t *testing.T) {
		s := testToeholdSync(t, provider, provider, syncTemplate())
		if err := s.Run(context.TODO()); err != nil {
			t.Fatalf("Run: %v", err)
		}
		owner := meta.GetControllerOf(getTemplate(t, s))
		if owner == nil || owner.UID != provider.UID {
			t.Errorf("owner = %v, want the provider", owner)
		}
	})

	// The spec already matching is not a reason to leave it unowned.
	t.Run("on a template whose spec already matches", func(t *testing.T) {
		s := testToeholdSync(t, provider, provider, syncTemplate())
		if err := s.Run(context.TODO()); err != nil {
			t.Fatalf("settle: %v", err)
		}
		template := getTemplate(t, s)
		template.OwnerReferences = nil
		if err := s.client.Update(context.TODO(), template); err != nil {
			t.Fatalf("clear ownership: %v", err)
		}

		if err := s.Run(context.TODO()); err != nil {
			t.Fatalf("Run: %v", err)
		}
		owner := meta.GetControllerOf(getTemplate(t, s))
		if owner == nil || owner.UID != provider.UID {
			t.Errorf("owner = %v, want the provider adopted it", owner)
		}
	})
}

// Writing placement the provider has not resolved yet would send the toehold
// controller off to build against empty strings.
func TestToeholdSyncWaitsForTheProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider func(*api.Provider)
		settings func()
	}{
		{
			name: "a blocked provider",
			provider: func(p *api.Provider) {
				p.Status.SetCondition(libcnd.Condition{Type: ConnectionTestFailed, Status: True, Category: Critical})
			},
		},
		{
			name:     "the connection has not been tested",
			provider: func(p *api.Provider) { p.Status.DeleteCondition(ConnectionTestSucceeded) },
		},
		{
			name:     "the inventory has not been built",
			provider: func(p *api.Provider) { p.Status.DeleteCondition(InventoryCreated) },
		},
		{
			name:     "no base disk image is configured",
			settings: func() { Settings.BaseDiskContainerImage = "" },
		},
		{
			name:     "no datastore is set",
			provider: func(p *api.Provider) { delete(p.Spec.Settings, api.ToeholdDatastore) },
		},
		{
			name:     "no folder is set",
			provider: func(p *api.Provider) { delete(p.Spec.Settings, api.ToeholdFolder) },
		},
		{
			name:     "no network is set",
			provider: func(p *api.Provider) { delete(p.Spec.Settings, api.ToeholdNetwork) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withSyncSettings(t)
			if tt.settings != nil {
				tt.settings()
			}
			provider := syncProvider()
			if tt.provider != nil {
				tt.provider(provider)
			}
			s := testToeholdSync(t, provider, provider, syncTemplate())
			before := getTemplate(t, s).ResourceVersion

			if err := s.Run(context.TODO()); err != nil {
				t.Fatalf("Run: %v", err)
			}

			template := getTemplate(t, s)
			if template.ResourceVersion != before {
				t.Errorf("the template was written, want it left alone")
			}
			if template.Spec.Datastore != "the-old-datastore" {
				t.Errorf("Datastore = %q, want it untouched", template.Spec.Datastore)
			}
		})
	}
}
