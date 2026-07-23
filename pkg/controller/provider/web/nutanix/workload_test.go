package nutanix

import (
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// TestWorkload_With verifies Workload inherits VM's field-copying behavior
// via embedding (Workload defines no With() override of its own).
func TestWorkload_With(t *testing.T) {
	m := &model.VM{
		Base:       model.Base{ID: "vm-1", Name: "web-server-rhel8"},
		UUID:       "vm-1",
		Cluster:    "cluster-1",
		PowerState: "ON",
	}

	r := &Workload{}
	r.With(m)

	if r.ID != m.ID || r.Name != m.Name || r.UUID != m.UUID || r.PowerState != m.PowerState {
		t.Errorf("expected Workload.With() to inherit VM's field copying, got %+v", r)
	}
}

func TestWorkload_Link(t *testing.T) {
	provider := &api.Provider{ObjectMeta: meta.ObjectMeta{UID: types.UID("provider-1")}}
	r := &Workload{}
	r.ID = "vm-1"
	r.Link(provider)

	if !strings.Contains(r.SelfLink, "provider-1") {
		t.Errorf("expected SelfLink to contain the provider UID, got %q", r.SelfLink)
	}
	if !strings.HasSuffix(r.SelfLink, "vm-1") {
		t.Errorf("expected SelfLink to end with the workload ID, got %q", r.SelfLink)
	}
	if !strings.Contains(r.SelfLink, "workloads") {
		t.Errorf("expected SelfLink to route through the workloads collection, got %q", r.SelfLink)
	}
}

func TestWorkload_IsWindows(t *testing.T) {
	tests := []struct {
		name           string
		guestOSID      string
		guestOSVersion string
		expected       bool
	}{
		{"windows guest os id", "windows_2019", "", true},
		{"windows in version string", "unknown", "Windows Server 2022 Datacenter", true},
		{"mixed case windows", "WinDoWs", "", true},
		{"linux guest", "rhel8_64Guest", "Red Hat Enterprise Linux 8.9", false},
		{"empty fields", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Workload{}
			r.GuestOSID = tt.guestOSID
			r.GuestOSVersion = tt.guestOSVersion
			if got := r.IsWindows(); got != tt.expected {
				t.Errorf("IsWindows() with GuestOSID=%q GuestOSVersion=%q = %v, want %v",
					tt.guestOSID, tt.guestOSVersion, got, tt.expected)
			}
		})
	}
}
