package copyappliance

import (
	"context"
	"slices"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testLog() logging.LevelLogger {
	return logging.WithName("copy-appliance-test")
}

func testReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	return &Reconciler{
		Reconciler: base.Reconciler{
			Client: testClient(t, objs...),
			Log:    testLog(),
		},
	}
}

// testClient is a cluster holding the given objects, which the runners reach
// the setup pods through.
func testClient(t *testing.T, objs ...runtime.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithRuntimeObjects(objs...).
		Build()
}

// testScheme knows the forklift kinds plus the core kinds the setup pod is
// made of.
func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		api.SchemeBuilder.AddToScheme,
		core.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatalf("AddToScheme: %v", err)
		}
	}
	return scheme
}

func testAppliance() *api.CopyAppliance {
	return &api.CopyAppliance{
		ObjectMeta: meta.ObjectMeta{
			Name:      "appliance",
			Namespace: "forklift",
			UID:       types.UID("11111111-2222-3333-4444-555555555555"),
		},
		Spec: api.CopyApplianceSpec{
			Provider:       core.ObjectReference{Namespace: "forklift", Name: "vsphere"},
			Secret:         core.ObjectReference{Namespace: "forklift", Name: "appliance-secret"},
			ContainerImage: "quay.io/kubev2v/nbd-container:latest",
			Datacenter:     "DC0",
			Datastore:      "datastore1",
			ResourcePool:   "/DC0/host/cluster/Resources",
			Folder:         "/DC0/vm",
			Template:       "/DC0/vm/appliance-template",
			AttachDisks: []api.AttachedDisk{
				{VMDKPath: "[datastore13] vm-a/disk-0.vmdk", Serial: "wwn-abc", DiskKey: 2000},
				{VMDKPath: "[datastore13] vm-b/disk-0.vmdk", Serial: "wwn-def", DiskKey: 2001},
			},
		},
	}
}

// The finalizer is the only thing keeping the CopyAppliance in the cluster
// while its VM is still up. Releasing it early leaves an appliance holding read
// locks on the source vmdks with nothing left to point at it.
func TestRemoveFinalizer(t *testing.T) {
	tests := []struct {
		name     string
		phase    string
		wantHeld bool
	}{
		{"the finalizer is held while teardown is in flight", api.PhaseWaitForDestroyVM, true},
		{"the finalizer is held when teardown has failed", api.PhaseTeardownFailed, true},
		{"the finalizer is released once teardown completes", api.PhaseTeardownCompleted, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			appliance := testAppliance()
			appliance.Finalizers = []string{api.CopyApplianceFinalizer}
			appliance.Status.Phase = tc.phase
			r := testReconciler(t, appliance)

			err := r.RemoveFinalizer(context.TODO(), appliance)
			if err != nil {
				t.Fatalf("RemoveFinalizer: %v", err)
			}

			held := slices.Contains(appliance.Finalizers, api.CopyApplianceFinalizer)
			if held != tc.wantHeld {
				t.Errorf("finalizer held = %v, want %v", held, tc.wantHeld)
			}
		})
	}
}

// A moRef means nothing outside the vCenter it came from. Acting on one from
// another vCenter would power on, or destroy, an unrelated VM.
func TestApplianceForgetForeignVM(t *testing.T) {
	r := &Reconciler{Reconciler: base.Reconciler{Log: testLog()}}

	t.Run("every field describing the VM is cleared together", func(t *testing.T) {
		appliance := testAppliance()
		appliance.Status.MoRef = "vm-42"
		appliance.Status.VCenterInstanceUUID = "uuid-a"
		appliance.Status.TaskRef = "task-7"
		appliance.Status.Phase = api.PhaseWaitForClone
		appliance.Status.Addresses = []api.ApplianceAddress{
			{Network: "VM Network", MAC: "00:50:56:01:02:03", IP: "192.0.2.10"},
		}
		appliance.Status.Exports = []api.ApplianceExport{
			{WWID: "wwn-abc", Port: 10809, Device: "/dev/sdb"},
		}

		r.forgetForeignVM(appliance, "uuid-b")

		status := appliance.Status
		if status.MoRef != "" ||
			status.VCenterInstanceUUID != "" ||
			status.TaskRef != "" ||
			status.Phase != "" ||
			status.Addresses != nil ||
			status.Exports != nil {
			t.Errorf("identity not fully cleared: %+v", status)
		}
	})

	tests := []struct {
		name        string
		vmID        string
		recorded    string
		connected   string
		wantForgot  bool
		description string
	}{
		{"same vCenter is kept", "vm-42", "uuid-a", "uuid-a", false, ""},
		{"different vCenter is forgotten", "vm-42", "uuid-a", "uuid-b", true, ""},
		{"unknown recorded UUID is kept", "vm-42", "", "uuid-b", false,
			"a VM recorded before the UUID was tracked must be adopted, not abandoned"},
		{"unknown connected UUID is kept", "vm-42", "uuid-a", "", false,
			"an unreadable connection UUID is not evidence of a different vCenter"},
		{"no VM recorded", "", "uuid-a", "uuid-b", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			appliance := testAppliance()
			appliance.Status.MoRef = tc.vmID
			appliance.Status.VCenterInstanceUUID = tc.recorded

			r.forgetForeignVM(appliance, tc.connected)

			forgot := appliance.Status.MoRef == "" && tc.vmID != ""
			if forgot != tc.wantForgot {
				t.Errorf("forgot = %v, want %v. %s", forgot, tc.wantForgot, tc.description)
			}
		})
	}
}

// The provider's copyApplianceTemplate check reports why its appliance failed, and this is
// the only place the reason is recorded. setConverging writes a Ready condition
// on every non-terminal phase, so the presence of one is not enough.
func TestFailureReason(t *testing.T) {
	r := &Reconciler{}

	failed := &api.CopyAppliance{}
	r.setFailed(failed, api.PhaseDeployFailed, "CloneFailed",
		liberr.New("the guest never reported an address"))
	if got := FailureReason(failed); got != "the guest never reported an address" {
		t.Errorf("FailureReason = %q, want the recorded message", got)
	}
	// Staging on a later reconcile must not hide the durable fault.
	failed.Status.BeginStagingConditions()
	if got := FailureReason(failed); got != "the guest never reported an address" {
		t.Errorf("FailureReason after BeginStaging = %q, want durable message", got)
	}
	failed.Status.EndStagingConditions()
	if got := FailureReason(failed); got != "the guest never reported an address" {
		t.Errorf("FailureReason after EndStaging = %q, want durable message", got)
	}
	// PhaseDeployFailed promotes Error → Critical; plan still needs the detail.
	failed.Status.SetCondition(libcnd.Condition{
		Type:     libcnd.Ready,
		Status:   libcnd.False,
		Reason:   api.PhaseDeployFailed,
		Category: libcnd.Critical,
		Message:  "task failed: missing privileges: Cryptographer.Encrypt",
		Durable:  true,
	})
	if got := FailureReason(failed); got != "task failed: missing privileges: Cryptographer.Encrypt" {
		t.Errorf("FailureReason for Critical = %q, want the detailed message", got)
	}

	converging := &api.CopyAppliance{}
	converging.Status.Phase = api.PhaseWaitForNetwork
	r.setConverging(converging, "waiting for an address")
	if got := FailureReason(converging); got == "waiting for an address" {
		t.Error("FailureReason returned a converging message as a failure")
	}

	if got := FailureReason(&api.CopyAppliance{}); got == "" {
		t.Error("FailureReason is empty for an appliance that recorded nothing")
	}
}
