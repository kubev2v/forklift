package copyappliance

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

func TestPendingExportRequest(t *testing.T) {
	disk := api.AttachedDisk{VMDKPath: "[ds] vm/disk-0.vmdk"}
	appliance := &api.CopyAppliance{
		Status: api.CopyApplianceStatus{
			Phase: api.PhaseDeployCompleted,
		},
	}
	if !PendingExportRequest(appliance) {
		t.Fatal("expected pending when DeployCompleted and AttachDisks cleared")
	}

	appliance.Spec.AttachDisks = []api.AttachedDisk{disk}
	if PendingExportRequest(appliance) {
		t.Fatal("did not expect pending when DeployCompleted and AttachDisks set")
	}

	appliance.Status.Phase = api.PhaseReleased
	if !PendingExportRequest(appliance) {
		t.Fatal("expected pending when Released and AttachDisks set")
	}

	appliance.Spec.AttachDisks = nil
	if PendingExportRequest(appliance) {
		t.Fatal("did not expect pending when Released and AttachDisks cleared")
	}

	appliance.Status.Phase = api.PhaseAttachDisks
	appliance.Spec.AttachDisks = []api.AttachedDisk{disk}
	if PendingExportRequest(appliance) {
		t.Fatal("did not expect pending while already in the export pipeline")
	}
}

func TestExportRunner_BeginSetsReleasePhase(t *testing.T) {
	appliance := &api.CopyAppliance{
		Status: api.CopyApplianceStatus{
			Phase: api.PhaseDeployCompleted,
		},
	}
	runner := ExportRunner{context: &ApplianceContext{Appliance: appliance}}
	if err := runner.begin(); err != nil {
		t.Fatalf("begin: %v", err)
	}
	if appliance.Status.Phase != api.PhaseReleaseDisks {
		t.Fatalf("phase = %q, want %q", appliance.Status.Phase, api.PhaseReleaseDisks)
	}
}

func TestExportRunner_BeginSetsAttachPhase(t *testing.T) {
	appliance := &api.CopyAppliance{
		Spec: api.CopyApplianceSpec{
			AttachDisks: []api.AttachedDisk{{VMDKPath: "[ds] vm/disk-0.vmdk"}},
		},
		Status: api.CopyApplianceStatus{
			Phase: api.PhaseReleased,
		},
	}
	runner := ExportRunner{context: &ApplianceContext{Appliance: appliance}}
	if err := runner.begin(); err != nil {
		t.Fatalf("begin: %v", err)
	}
	if appliance.Status.Phase != api.PhaseAttachDisks {
		t.Fatalf("phase = %q, want %q", appliance.Status.Phase, api.PhaseAttachDisks)
	}
}
