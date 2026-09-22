package ocp

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	corev1 "k8s.io/api/core/v1"
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

func TestNICNetworkRefs_NoDuplicates(t *testing.T) {
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
					Source:      api.NetworkSourceRef{Ref: ref.Ref{Namespace: "test-ns", Name: "net-1"}},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
				{
					Source:      api.NetworkSourceRef{Ref: ref.Ref{Namespace: "test-ns", Name: "net-2"}},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-b"},
				},
			},
		},
	}

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "nic-refs"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: plan},
	}
	nicRefs, err := validator.NICNetworkRefs(ref.Ref{Name: "test-vm", Namespace: "test-ns"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nicRefs) != 2 {
		t.Fatalf("expected 2 NIC refs, got %d", len(nicRefs))
	}

	if planbase.ValidatePodNetworkDuplicates(nicRefs, plan.Map.Network) {
		t.Errorf("expected no pod duplicates")
	}
}

func TestNICNetworkRefs_TwoNICsSameNAD(t *testing.T) {
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
					Source:      api.NetworkSourceRef{Ref: ref.Ref{Namespace: "test-ns", Name: "net-1"}},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
				{
					Source:      api.NetworkSourceRef{Ref: ref.Ref{Namespace: "test-ns", Name: "net-2"}},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
			},
		},
	}

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "nic-refs"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: plan},
	}
	nicRefs, err := validator.NICNetworkRefs(ref.Ref{Name: "test-vm", Namespace: "test-ns"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if planbase.ValidatePodNetworkDuplicates(nicRefs, plan.Map.Network) {
		t.Errorf("expected no pod duplicate when two NICs map to same NAD")
	}
}

func TestNICNetworkRefs_TwoNICsSameSourceNetwork(t *testing.T) {
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
					Source:      api.NetworkSourceRef{Ref: ref.Ref{Namespace: "test-ns", Name: "net-1"}},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
			},
		},
	}

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "nic-refs"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: plan},
	}
	nicRefs, err := validator.NICNetworkRefs(ref.Ref{Name: "test-vm", Namespace: "test-ns"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if planbase.ValidatePodNetworkDuplicates(nicRefs, plan.Map.Network) {
		t.Errorf("expected no pod duplicate when two NICs on same source map to same NAD")
	}
}

func TestNICNetworkRefs_VMOnlyUsesOneOfDuplicateMappings(t *testing.T) {
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
					Source:      api.NetworkSourceRef{Ref: ref.Ref{Namespace: "test-ns", Name: "net-1"}},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
				{
					Source:      api.NetworkSourceRef{Ref: ref.Ref{Namespace: "test-ns", Name: "net-2"}},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
			},
		},
	}

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "nic-refs"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: plan},
	}
	nicRefs, err := validator.NICNetworkRefs(ref.Ref{Name: "test-vm", Namespace: "test-ns"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if planbase.ValidatePodNetworkDuplicates(nicRefs, plan.Map.Network) {
		t.Errorf("expected no pod duplicate (VM only uses one of the duplicate mappings)")
	}
}

func TestNICNetworkRefs_PodAndMultus(t *testing.T) {
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
					Source:      api.NetworkSourceRef{Ref: ref.Ref{Type: "pod"}},
					Destination: api.DestinationNetwork{Type: "pod"},
				},
				{
					Source:      api.NetworkSourceRef{Ref: ref.Ref{Namespace: "test-ns", Name: "net-1"}},
					Destination: api.DestinationNetwork{Type: "multus", Namespace: "ns1", Name: "nad-a"},
				},
			},
		},
	}

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "nic-refs"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: plan},
	}
	nicRefs, err := validator.NICNetworkRefs(ref.Ref{Name: "test-vm", Namespace: "test-ns"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nicRefs) != 2 {
		t.Fatalf("expected 2 NIC refs (pod + multus), got %d", len(nicRefs))
	}

	if planbase.ValidatePodNetworkDuplicates(nicRefs, plan.Map.Network) {
		t.Errorf("expected no pod duplicate (single pod network and single multus NAD)")
	}
}

func TestNICNetworkRefs_VMNotFound(t *testing.T) {
	client := newFakeClient().Build()

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "nic-refs"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: &api.Plan{}},
	}
	_, err := validator.NICNetworkRefs(ref.Ref{Name: "nonexistent", Namespace: "test-ns"})
	if err == nil {
		t.Errorf("expected error for missing VM, got nil")
	}
}

func TestPVCNameTemplate_UsesSpecTargetName(t *testing.T) {
	vm := &cnv.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "source-vm", Namespace: "test-ns"},
		Spec: cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				Spec: cnv.VirtualMachineInstanceSpec{
					Volumes: []cnv.Volume{
						{Name: "vol-0", VolumeSource: cnv.VolumeSource{
							PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
								PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "src-pvc"},
							},
						}},
					},
				},
			},
		},
	}
	client := newFakeClient(vm).Build()

	plan := &api.Plan{
		ObjectMeta: metav1.ObjectMeta{Name: "test-plan"},
		Spec: api.PlanSpec{
			VMs: []planapi.VM{{
				Ref:        ref.Ref{ID: "vm-id", Name: "source-vm", Namespace: "test-ns"},
				TargetName: "my-target",
			}},
		},
	}

	validator := &Validator{
		log:          logging.WithName("test").WithValues("test", "pvc-template"),
		sourceClient: client,
		Context:      &plancontext.Context{Plan: plan},
	}

	// Template that only produces a valid name when .TargetVmName == "my-target".
	tmpl := `{{if eq .TargetVmName "my-target"}}ok-{{.DiskIndex}}{{else}}INVALID{{end}}`

	ok, err := validator.PVCNameTemplate(
		ref.Ref{ID: "vm-id", Name: "source-vm", Namespace: "test-ns"},
		tmpl,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected ok=true: validator should resolve spec.targetName via ResolveTargetVmName")
	}
}
