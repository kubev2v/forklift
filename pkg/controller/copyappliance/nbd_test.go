package copyappliance

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
)

func TestNbdURI(t *testing.T) {
	if got := NbdURI("10.0.0.5", 10809, false); got != "nbd://10.0.0.5:10809" {
		t.Fatalf("NbdURI() = %q", got)
	}
	if got := NbdURI("10.0.0.5", 10809, true); got != "nbds://10.0.0.5:10809" {
		t.Fatalf("NbdURI(ssl) = %q", got)
	}
}

func TestExportNbdConnections(t *testing.T) {
	appliance := &api.CopyAppliance{
		Spec: api.CopyApplianceSpec{
			AttachDisks: []api.AttachedDisk{
				{VMDKPath: "[ds] vm/disk-0.vmdk"},
			},
		},
		Status: api.CopyApplianceStatus{
			Addresses: []api.ApplianceAddress{{IP: "10.0.0.5"}},
			Exports: []api.ApplianceExport{
				{VMDKPath: "[ds] vm/disk-0.vmdk", Port: 10809},
			},
		},
	}
	connections, err := ExportNbdConnections(appliance, false)
	if err != nil {
		t.Fatalf("ExportNbdConnections() error: %v", err)
	}
	if connections["[ds] vm/disk-0.vmdk"] != "nbd://10.0.0.5:10809" {
		t.Fatalf("unexpected connection map: %#v", connections)
	}
	connections, err = ExportNbdConnections(appliance, true)
	if err != nil {
		t.Fatalf("ExportNbdConnections(ssl) error: %v", err)
	}
	if connections["[ds] vm/disk-0.vmdk"] != "nbds://10.0.0.5:10809" {
		t.Fatalf("unexpected ssl connection map: %#v", connections)
	}
}

func TestExportNbdConnectionsWarmSnapshotPath(t *testing.T) {
	appliance := &api.CopyAppliance{
		Spec: api.CopyApplianceSpec{
			AttachDisks: []api.AttachedDisk{
				{VMDKPath: "[ds] vm/disk-0-000003.vmdk"},
			},
		},
		Status: api.CopyApplianceStatus{
			Addresses: []api.ApplianceAddress{{IP: "10.0.0.5"}},
			Exports: []api.ApplianceExport{
				{VMDKPath: "[ds] vm/disk-0-000003.vmdk", Port: 10809},
			},
		},
	}
	connections, err := ExportNbdConnections(appliance, false)
	if err != nil {
		t.Fatalf("ExportNbdConnections() error: %v", err)
	}
	if connections["[ds] vm/disk-0.vmdk"] != "nbd://10.0.0.5:10809" {
		t.Fatalf("expected base path mapping, got %#v", connections)
	}
}

func TestIsDeployReady(t *testing.T) {
	appliance := &api.CopyAppliance{
		Status: api.CopyApplianceStatus{
			Phase: api.PhaseDeployCompleted,
			Conditions: libcnd.Conditions{
				List: []libcnd.Condition{{
					Type:   libcnd.Ready,
					Status: libcnd.True,
				}},
			},
		},
	}
	if !IsDeployReady(appliance) {
		t.Fatal("expected deploy ready appliance")
	}
}
