package hyperv

import (
	"fmt"
	"strings"
	"testing"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	webbase "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
)

func TestParseStateString(t *testing.T) {
	c := &Client{}
	tests := []struct {
		name     string
		raw      string
		expected planapi.VMPowerState
	}{
		{"off lowercase", "Off", planapi.VMPowerStateOff},
		{"off with whitespace", "  Off\r\n", planapi.VMPowerStateOff},
		{"running", "Running", planapi.VMPowerStateOn},
		{"running with whitespace", "\n Running \n", planapi.VMPowerStateOn},
		{"unknown state", "Paused", planapi.VMPowerStateUnknown},
		{"empty string", "", planapi.VMPowerStateUnknown},
		{"shutoff variant", "TurnedOff", planapi.VMPowerStateOff},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.parseStateString(tc.raw)
			if got != tc.expected {
				t.Errorf("parseStateString(%q) = %q, want %q", tc.raw, got, tc.expected)
			}
		})
	}
}

func TestPowerStateFromInventory(t *testing.T) {
	c := &Client{}
	tests := []struct {
		name     string
		state    string
		expected planapi.VMPowerState
	}{
		{"on", model.PowerStateOn, planapi.VMPowerStateOn},
		{"off", model.PowerStateOff, planapi.VMPowerStateOff},
		{"paused", model.PowerStatePaused, planapi.VMPowerStateUnknown},
		{"empty", "", planapi.VMPowerStateUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := &hyperv.VM{PowerState: tc.state}
			got := c.powerStateFromInventory(vm)
			if got != tc.expected {
				t.Errorf("powerStateFromInventory(state=%q) = %q, want %q",
					tc.state, got, tc.expected)
			}
		})
	}
}

// ---- GetSnapshotDeltas ----

// deltaStubDriver returns canned PowerShell output for RunOnNode.
type deltaStubDriver struct {
	driver.HyperVDriver
	out string
	err error
}

func (d *deltaStubDriver) IsAlive() (bool, error)                { return true, nil }
func (d *deltaStubDriver) RunOnNode(_, _ string) (string, error) { return d.out, d.err }

// deltaStubInventory serves a fixed VM through Find.
type deltaStubInventory struct {
	webbase.Client
	vm *hyperv.VM
}

func (s *deltaStubInventory) Find(resource interface{}, _ webbase.Ref) error {
	vm, ok := resource.(*hyperv.VM)
	if !ok {
		return fmt.Errorf("deltaStubInventory: unsupported resource %T", resource)
	}
	*vm = *s.vm
	return nil
}

// newDeltaTestClient builds a Client serving the given disks and PowerShell
// output. hyperv.VM's Name/Disks are promoted fields, so they are assigned
// rather than set in a composite literal.
func newDeltaTestClient(disks []model.Disk, psOutput string) *Client {
	vm := &hyperv.VM{}
	vm.Name = "win2019"
	vm.Host = "node1"
	vm.Disks = disks
	warmPlan := &v1beta1.Plan{Spec: v1beta1.PlanSpec{Type: v1beta1.MigrationWarm}}
	return &Client{
		Context: &plancontext.Context{
			Plan:   warmPlan,
			Source: plancontext.Source{Inventory: &deltaStubInventory{vm: vm}},
		},
		driver: &deltaStubDriver{out: psOutput},
	}
}

func TestGetSnapshotDeltasMatchesDisksByRootPath(t *testing.T) {
	vmRef := ref.Ref{ID: "vm-1"}
	// Order deliberately differs from inventory: delta for data.vhdx comes
	// first. Index-based mapping would attach it to disk-0.
	psOutput := `[{"Active":"C:\\VHDX\\data_A1.avhdx","Parent":"C:\\VHDX\\data.vhdx","Root":"C:\\VHDX\\data.vhdx"},` +
		`{"Active":"C:\\VHDX\\os_A1.avhdx","Parent":"C:\\VHDX\\os.vhdx","Root":"C:\\VHDX\\os.vhdx"}]`
	disks := []model.Disk{
		{Base: model.Base{ID: "disk-0"}, WindowsPath: `C:\VHDX\os.vhdx`},
		{Base: model.Base{ID: "disk-1"}, WindowsPath: `C:\VHDX\data.vhdx`},
	}

	deltas, err := newDeltaTestClient(disks, psOutput).GetSnapshotDeltas(vmRef, "", nil)
	if err != nil {
		t.Fatalf("GetSnapshotDeltas: %v", err)
	}
	if len(deltas) != 2 {
		t.Fatalf("got %d deltas, want 2: %v", len(deltas), deltas)
	}
	if want := `{"diskPaths":{"Active":"C:\\VHDX\\data_A1.avhdx","Parent":"C:\\VHDX\\data.vhdx","Root":"C:\\VHDX\\data.vhdx"}}`; deltas["disk-1"] != want {
		t.Errorf("disk-1 delta = %s, want %s", deltas["disk-1"], want)
	}
	if want := `{"diskPaths":{"Active":"C:\\VHDX\\os_A1.avhdx","Parent":"C:\\VHDX\\os.vhdx","Root":"C:\\VHDX\\os.vhdx"}}`; deltas["disk-0"] != want {
		t.Errorf("disk-0 delta = %s, want %s", deltas["disk-0"], want)
	}
}

func TestGetSnapshotDeltasSingleObjectOutput(t *testing.T) {
	vmRef := ref.Ref{ID: "vm-1"}
	psOutput := `{"Active":"C:\\VHDX\\os_A1.avhdx","Parent":"C:\\VHDX\\os.vhdx","Root":"C:\\VHDX\\os.vhdx"}`

	c := newDeltaTestClient([]model.Disk{
		{Base: model.Base{ID: "disk-0"}, WindowsPath: `C:\VHDX\os.vhdx`},
	}, psOutput)

	deltas, err := c.GetSnapshotDeltas(vmRef, "", nil)
	if err != nil {
		t.Fatalf("GetSnapshotDeltas: %v", err)
	}
	if _, ok := deltas["disk-0"]; !ok {
		t.Fatalf("disk-0 missing from deltas: %v", deltas)
	}
}

func TestGetSnapshotDeltasRejectsUnknownRoot(t *testing.T) {
	vmRef := ref.Ref{ID: "vm-1"}
	psOutput := `[{"Active":"C:\\VHDX\\new_A1.avhdx","Parent":"C:\\VHDX\\new.vhdx","Root":"C:\\VHDX\\new.vhdx"}]`

	c := newDeltaTestClient([]model.Disk{
		{Base: model.Base{ID: "disk-0"}, WindowsPath: `C:\VHDX\os.vhdx`},
	}, psOutput)

	_, err := c.GetSnapshotDeltas(vmRef, "", nil)
	if err == nil {
		t.Fatal("expected an error for a checkpoint disk matching no inventory disk")
	}
	if !strings.Contains(err.Error(), "matches no inventory disk") {
		t.Errorf("error %q does not mention the unmatched disk", err)
	}
}

func TestGetSnapshotDeltasPropagatesRootGetVHDFailure(t *testing.T) {
	c := newDeltaTestClient([]model.Disk{
		{Base: model.Base{ID: "disk-0"}, WindowsPath: `C:\VHDX\os.vhdx`},
	}, "")
	c.driver = &deltaStubDriver{err: fmt.Errorf("Get-VHD failed for root disk C:\\VHDX\\os.vhdx")}

	_, err := c.GetSnapshotDeltas(ref.Ref{ID: "vm-1"}, "", nil)
	if err == nil {
		t.Fatal("expected Get-VHD root failure to propagate")
	}
	if !strings.Contains(err.Error(), "getting checkpoint disk paths") {
		t.Errorf("error %q missing GetSnapshotDeltas wrap", err)
	}
	if !strings.Contains(err.Error(), "Get-VHD failed for root disk") {
		t.Errorf("error %q missing root Get-VHD cause", err)
	}
}

func TestGetSnapshotDeltasPropagatesChainGetVHDFailure(t *testing.T) {
	c := newDeltaTestClient([]model.Disk{
		{Base: model.Base{ID: "disk-0"}, WindowsPath: `C:\VHDX\os.vhdx`},
	}, "")
	c.driver = &deltaStubDriver{err: fmt.Errorf("Get-VHD failed for chain disk C:\\VHDX\\os.vhdx")}

	_, err := c.GetSnapshotDeltas(ref.Ref{ID: "vm-1"}, "", nil)
	if err == nil {
		t.Fatal("expected Get-VHD chain failure to propagate")
	}
	if !strings.Contains(err.Error(), "getting checkpoint disk paths") {
		t.Errorf("error %q missing GetSnapshotDeltas wrap", err)
	}
	if !strings.Contains(err.Error(), "Get-VHD failed for chain disk") {
		t.Errorf("error %q missing chain Get-VHD cause", err)
	}
}

// Pre-existing differencing disk: inventory records the attached child.vhdx,
// not the oldest template. Root must be that attached identity.
func TestGetSnapshotDeltasMatchesDifferencingAttachedPath(t *testing.T) {
	vmRef := ref.Ref{ID: "vm-1"}
	psOutput := `{"Active":"C:\\VHDX\\child_A1.avhdx","Parent":"C:\\VHDX\\child.vhdx","Root":"C:\\VHDX\\child.vhdx"}`

	c := newDeltaTestClient([]model.Disk{
		{Base: model.Base{ID: "disk-0"}, WindowsPath: `C:\VHDX\child.vhdx`},
	}, psOutput)

	deltas, err := c.GetSnapshotDeltas(vmRef, "", nil)
	if err != nil {
		t.Fatalf("GetSnapshotDeltas: %v", err)
	}
	if _, ok := deltas["disk-0"]; !ok {
		t.Fatalf("disk-0 missing from deltas: %v", deltas)
	}
}

// Inventory path is an attached AVHDX; Root walked to the base .vhdx —
// Parent must still match.
func TestGetSnapshotDeltasMatchesAttachedAVHDXInventoryPath(t *testing.T) {
	vmRef := ref.Ref{ID: "vm-1"}
	psOutput := `{"Active":"C:\\VHDX\\child_A2.avhdx","Parent":"C:\\VHDX\\child.avhdx","Root":"C:\\VHDX\\template.vhdx"}`

	c := newDeltaTestClient([]model.Disk{
		{Base: model.Base{ID: "disk-0"}, WindowsPath: `C:\VHDX\child.avhdx`},
	}, psOutput)

	deltas, err := c.GetSnapshotDeltas(vmRef, "", nil)
	if err != nil {
		t.Fatalf("GetSnapshotDeltas: %v", err)
	}
	if _, ok := deltas["disk-0"]; !ok {
		t.Fatalf("disk-0 missing from deltas: %v", deltas)
	}
}

// Later checkpoint: Parent is a forklift AVHDX, inventory still names the
// originally attached mid-chain AVHDX — match via Chain.
func TestGetSnapshotDeltasMatchesAVHDXViaChainAcrossCheckpoints(t *testing.T) {
	vmRef := ref.Ref{ID: "vm-1"}
	psOutput := `{"Active":"C:\\VHDX\\child_A3.avhdx","Parent":"C:\\VHDX\\child_A2.avhdx","Root":"C:\\VHDX\\template.vhdx",` +
		`"Chain":["C:\\VHDX\\child_A3.avhdx","C:\\VHDX\\child_A2.avhdx","C:\\VHDX\\child.avhdx","C:\\VHDX\\template.vhdx"]}`

	c := newDeltaTestClient([]model.Disk{
		{Base: model.Base{ID: "disk-0"}, WindowsPath: `C:\VHDX\child.avhdx`},
	}, psOutput)

	deltas, err := c.GetSnapshotDeltas(vmRef, "", nil)
	if err != nil {
		t.Fatalf("GetSnapshotDeltas: %v", err)
	}
	if _, ok := deltas["disk-0"]; !ok {
		t.Fatalf("disk-0 missing from deltas: %v", deltas)
	}
}

func TestGetSnapshotDeltasRejectsMissingDisk(t *testing.T) {
	vmRef := ref.Ref{ID: "vm-1"}
	// Only one PowerShell record for a two-disk VM.
	psOutput := `{"Active":"C:\\VHDX\\os_A1.avhdx","Parent":"C:\\VHDX\\os.vhdx","Root":"C:\\VHDX\\os.vhdx"}`

	c := newDeltaTestClient([]model.Disk{
		{Base: model.Base{ID: "disk-0"}, WindowsPath: `C:\VHDX\os.vhdx`},
		{Base: model.Base{ID: "disk-1"}, WindowsPath: `C:\VHDX\data.vhdx`},
	}, psOutput)

	_, err := c.GetSnapshotDeltas(vmRef, "", nil)
	if err == nil {
		t.Fatal("expected an error for a disk with no checkpoint delta")
	}
	if !strings.Contains(err.Error(), "disk-1") {
		t.Errorf("error %q does not name the missing disk", err)
	}
}
