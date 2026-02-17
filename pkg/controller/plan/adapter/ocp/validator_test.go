package ocp

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cnv "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMacConflicts_SkipsCheckForColdMigrations(t *testing.T) {
	coldMigrationTypes := []api.MigrationType{
		api.MigrationCold,
		"", // Default migration type
	}

	for _, migrationType := range coldMigrationTypes {
		t.Run("migration_type_"+string(migrationType), func(t *testing.T) {
			// Create validator with cold migration type
			validator := &Validator{
				log: logging.WithName("test").WithValues("test", "mac-conflicts"),
				Context: &plancontext.Context{
					Plan: &api.Plan{
						Spec: api.PlanSpec{
							Type: migrationType,
						},
					},
				},
			}

			// Mock VM reference
			vmRef := ref.Ref{
				ID:        "test-vm-id",
				Name:      "test-vm",
				Namespace: "test-ns",
			}

			// Call MacConflicts - should return empty result without checking inventory
			conflicts, err := validator.MacConflicts(vmRef)

			// Should not error and should return empty conflicts
			if err != nil {
				t.Errorf("Cold migration should not error, got: %v", err)
			}
			if len(conflicts) != 0 {
				t.Errorf("Cold migration should return no conflicts, got %d conflicts", len(conflicts))
			}

			t.Logf("✓ %s migration correctly skipped MAC conflict check", string(migrationType))
		})
	}
}

func TestMacConflicts_BehaviorDocumentation(t *testing.T) {
	// This test documents the expected behavior without testing implementation details
	testCases := []struct {
		migrationType    api.MigrationType
		description      string
		expectsInventory bool
	}{
		{
			migrationType:    api.MigrationCold,
			description:      "Cold migration shuts down source VM, no MAC conflicts possible",
			expectsInventory: false,
		},
		{
			migrationType:    "",
			description:      "Default migration is cold, no MAC conflicts possible",
			expectsInventory: false,
		},
		{
			migrationType:    api.MigrationLive,
			description:      "Live migration keeps source VM running, MAC conflicts possible",
			expectsInventory: true,
		},
	}

	for _, tc := range testCases {
		t.Run("documents_"+string(tc.migrationType), func(t *testing.T) {
			t.Logf("Migration type '%s': %s", tc.migrationType, tc.description)
			if tc.expectsInventory {
				t.Logf("  → Should check destination inventory for MAC conflicts")
			} else {
				t.Logf("  → Should skip MAC conflict check entirely")
			}
		})
	}
}

func newFakeClient(objs ...runtime.Object) *fake.ClientBuilder {
	scheme := runtime.NewScheme()
	_ = cnv.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...)
}

func TestDuplicateNAD_NilNetworkMap(t *testing.T) {
	vm := &cnv.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vm", Namespace: "test-ns"},
	}
	client := newFakeClient(vm).Build()

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "dup-nad"),
		sourceClient: client,
		Context: &plancontext.Context{
			Plan: &api.Plan{},
		},
	}
	ok, err := validator.DuplicateNAD(ref.Ref{Name: "test-vm", Namespace: "test-ns"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ok {
		t.Errorf("expected ok=false (zero-value return when map is nil), got true")
	}
}

func TestDuplicateNAD_NoDuplicates(t *testing.T) {
	vm := &cnv.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vm", Namespace: "test-ns"},
		Spec: cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				Spec: cnv.VirtualMachineInstanceSpec{
					Networks: []cnv.Network{
						{Name: "nic1", NetworkSource: cnv.NetworkSource{Multus: &cnv.MultusNetwork{NetworkName: "test-ns/net-1"}}},
						{Name: "nic2", NetworkSource: cnv.NetworkSource{Multus: &cnv.MultusNetwork{NetworkName: "test-ns/net-2"}}},
					},
				},
			},
		},
	}
	client := newFakeClient(vm).Build()

	plan := &api.Plan{}
	plan.Referenced.Map.Network = &api.NetworkMap{
		Spec: api.NetworkMapSpec{
			Map: []api.NetworkPair{
				{
					Source:      ref.Ref{Namespace: "test-ns", Name: "net-1"},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
				{
					Source:      ref.Ref{Namespace: "test-ns", Name: "net-2"},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-b"},
				},
			},
		},
	}

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "dup-nad"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: plan},
	}
	ok, err := validator.DuplicateNAD(ref.Ref{Name: "test-vm", Namespace: "test-ns"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !ok {
		t.Errorf("expected ok=true (no duplicates), got false")
	}
}

func TestDuplicateNAD_TwoNICsSameNAD(t *testing.T) {
	vm := &cnv.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vm", Namespace: "test-ns"},
		Spec: cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				Spec: cnv.VirtualMachineInstanceSpec{
					Networks: []cnv.Network{
						{Name: "nic1", NetworkSource: cnv.NetworkSource{Multus: &cnv.MultusNetwork{NetworkName: "test-ns/net-1"}}},
						{Name: "nic2", NetworkSource: cnv.NetworkSource{Multus: &cnv.MultusNetwork{NetworkName: "test-ns/net-2"}}},
					},
				},
			},
		},
	}
	client := newFakeClient(vm).Build()

	plan := &api.Plan{}
	plan.Referenced.Map.Network = &api.NetworkMap{
		Spec: api.NetworkMapSpec{
			Map: []api.NetworkPair{
				{
					Source:      ref.Ref{Namespace: "test-ns", Name: "net-1"},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
				{
					Source:      ref.Ref{Namespace: "test-ns", Name: "net-2"},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
			},
		},
	}

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "dup-nad"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: plan},
	}
	ok, err := validator.DuplicateNAD(ref.Ref{Name: "test-vm", Namespace: "test-ns"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ok {
		t.Errorf("expected ok=false (two NICs map to same NAD), got true")
	}
}

func TestDuplicateNAD_TwoNICsSameSourceNetwork(t *testing.T) {
	// Two NICs on the same source network → both map to same destination NAD
	vm := &cnv.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vm", Namespace: "test-ns"},
		Spec: cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				Spec: cnv.VirtualMachineInstanceSpec{
					Networks: []cnv.Network{
						{Name: "nic1", NetworkSource: cnv.NetworkSource{Multus: &cnv.MultusNetwork{NetworkName: "test-ns/net-1"}}},
						{Name: "nic2", NetworkSource: cnv.NetworkSource{Multus: &cnv.MultusNetwork{NetworkName: "test-ns/net-1"}}},
					},
				},
			},
		},
	}
	client := newFakeClient(vm).Build()

	plan := &api.Plan{}
	plan.Referenced.Map.Network = &api.NetworkMap{
		Spec: api.NetworkMapSpec{
			Map: []api.NetworkPair{
				{
					Source:      ref.Ref{Namespace: "test-ns", Name: "net-1"},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
			},
		},
	}

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "dup-nad"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: plan},
	}
	ok, err := validator.DuplicateNAD(ref.Ref{Name: "test-vm", Namespace: "test-ns"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ok {
		t.Errorf("expected ok=false (two NICs on same source network → same NAD), got true")
	}
}

func TestDuplicateNAD_VMOnlyUsesOneOfDuplicateMappings(t *testing.T) {
	// Plan has two source networks mapping to the same NAD, but the VM only uses one.
	// This should pass — it's per-VM, not per-plan.
	vm := &cnv.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vm", Namespace: "test-ns"},
		Spec: cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				Spec: cnv.VirtualMachineInstanceSpec{
					Networks: []cnv.Network{
						{Name: "nic1", NetworkSource: cnv.NetworkSource{Multus: &cnv.MultusNetwork{NetworkName: "test-ns/net-1"}}},
					},
				},
			},
		},
	}
	client := newFakeClient(vm).Build()

	plan := &api.Plan{}
	plan.Referenced.Map.Network = &api.NetworkMap{
		Spec: api.NetworkMapSpec{
			Map: []api.NetworkPair{
				{
					Source:      ref.Ref{Namespace: "test-ns", Name: "net-1"},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
				{
					Source:      ref.Ref{Namespace: "test-ns", Name: "net-2"},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
			},
		},
	}

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "dup-nad"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: plan},
	}
	ok, err := validator.DuplicateNAD(ref.Ref{Name: "test-vm", Namespace: "test-ns"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !ok {
		t.Errorf("expected ok=true (VM only uses one of the duplicate mappings), got false")
	}
}

func TestDuplicateNAD_SkipsPodAndIgnored(t *testing.T) {
	vm := &cnv.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vm", Namespace: "test-ns"},
		Spec: cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				Spec: cnv.VirtualMachineInstanceSpec{
					Networks: []cnv.Network{
						{Name: "default", NetworkSource: cnv.NetworkSource{Pod: &cnv.PodNetwork{}}},
						{Name: "nic1", NetworkSource: cnv.NetworkSource{Multus: &cnv.MultusNetwork{NetworkName: "test-ns/net-1"}}},
					},
				},
			},
		},
	}
	client := newFakeClient(vm).Build()

	plan := &api.Plan{}
	plan.Referenced.Map.Network = &api.NetworkMap{
		Spec: api.NetworkMapSpec{
			Map: []api.NetworkPair{
				{
					Source:      ref.Ref{Type: "pod"},
					Destination: api.DestinationNetwork{Type: "pod"},
				},
				{
					Source:      ref.Ref{Namespace: "test-ns", Name: "net-1"},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
			},
		},
	}

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "dup-nad"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: plan},
	}
	ok, err := validator.DuplicateNAD(ref.Ref{Name: "test-vm", Namespace: "test-ns"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !ok {
		t.Errorf("expected ok=true (pod networks should not count), got false")
	}
}

func TestDuplicateNAD_VMNotFound(t *testing.T) {
	client := newFakeClient().Build()

	plan := &api.Plan{}
	plan.Referenced.Map.Network = &api.NetworkMap{
		Spec: api.NetworkMapSpec{
			Map: []api.NetworkPair{},
		},
	}

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "dup-nad"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: plan},
	}
	ok, err := validator.DuplicateNAD(ref.Ref{Name: "nonexistent", Namespace: "test-ns"})
	if err == nil {
		t.Errorf("expected error for missing VM, got nil")
	}
	if ok {
		t.Errorf("expected ok=false for missing VM, got true")
	}
}
