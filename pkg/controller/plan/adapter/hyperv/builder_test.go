package hyperv

import (
	"testing"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildNICKeys(t *testing.T) {
	tests := []struct {
		name                  string
		nics                  []hyperv.NIC
		vlanQualifiedNetworks map[string]bool
		expected              []string
	}{
		{
			name: "single NIC per network, no VLAN disambiguation",
			nics: []hyperv.NIC{
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 100},
				{Network: hyperv.Ref{ID: "net-b"}, VlanId: 200},
			},
			vlanQualifiedNetworks: map[string]bool{"net-a": true},
			expected:              []string{"net-a", "net-b"},
		},
		{
			name: "multiple NICs same network with different VLANs and VLAN-qualified map",
			nics: []hyperv.NIC{
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 100},
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 200},
			},
			vlanQualifiedNetworks: map[string]bool{"net-a": true},
			expected:              []string{"net-a/100", "net-a/200"},
		},
		{
			name: "multiple NICs same network but NO VLAN-qualified map (backward compat)",
			nics: []hyperv.NIC{
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 100},
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 200},
			},
			vlanQualifiedNetworks: map[string]bool{},
			expected:              []string{"net-a", "net-a"},
		},
		{
			name: "multiple NICs same network, one untagged (VlanId=0)",
			nics: []hyperv.NIC{
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 100},
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 0},
			},
			vlanQualifiedNetworks: map[string]bool{"net-a": true},
			expected:              []string{"net-a/100", "net-a"},
		},
		{
			name: "single NIC no VLAN",
			nics: []hyperv.NIC{
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 0},
			},
			vlanQualifiedNetworks: map[string]bool{},
			expected:              []string{"net-a"},
		},
		{
			name: "mixed: one network shared with VLAN map, another unique",
			nics: []hyperv.NIC{
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 100},
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 200},
				{Network: hyperv.Ref{ID: "net-b"}, VlanId: 50},
			},
			vlanQualifiedNetworks: map[string]bool{"net-a": true},
			expected:              []string{"net-a/100", "net-a/200", "net-b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			count := nicNetworkCount(tc.nics)
			keys := buildNICKeys(tc.nics, count, tc.vlanQualifiedNetworks)

			if len(keys) != len(tc.expected) {
				t.Fatalf("expected %d keys, got %d", len(tc.expected), len(keys))
			}
			for i, key := range keys {
				if key != tc.expected[i] {
					t.Errorf("keys[%d] = %q, want %q", i, key, tc.expected[i])
				}
			}
		})
	}
}

func TestBuildPairKey(t *testing.T) {
	tests := []struct {
		name         string
		networkID    string
		vlan         string
		networkCount map[string]int
		expected     string
	}{
		{
			name:         "no VLAN set",
			networkID:    "net-a",
			vlan:         "",
			networkCount: map[string]int{"net-a": 2},
			expected:     "net-a",
		},
		{
			name:         "VLAN set but only one NIC on network (no disambiguation needed)",
			networkID:    "net-a",
			vlan:         "100",
			networkCount: map[string]int{"net-a": 1},
			expected:     "net-a",
		},
		{
			name:         "VLAN set and multiple NICs on network",
			networkID:    "net-a",
			vlan:         "100",
			networkCount: map[string]int{"net-a": 2},
			expected:     "net-a/100",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := buildPairKey(tc.networkID, tc.vlan, tc.networkCount)
			if key != tc.expected {
				t.Errorf("buildPairKey(%q, %q, ...) = %q, want %q",
					tc.networkID, tc.vlan, key, tc.expected)
			}
		})
	}
}

var builderLog = logging.WithName("hyperv-builder-test")

var _ = Describe("HyperV builder", func() {
	Context("mapMacStaticIps with networkIPMode filtering", func() {
		It("should skip NICs with mode 'none'", func() {
			b := createBuilder()
			vm := &model.VM{
				GuestOS: "Windows Server 2019",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "172.29.3.193", Origin: hyperv.OriginManual, PrefixLength: 16, Gateway: "172.29.3.1"},
					{MAC: "00:15:5D:01:02:04", IP: "172.29.3.194", Origin: hyperv.OriginManual, PrefixLength: 16, Gateway: "172.29.3.1"},
				},
			}
			modeByMAC := map[string]string{
				"00:15:5D:01:02:03": "preserve",
				"00:15:5D:01:02:04": "none",
			}
			result := b.mapMacStaticIps(vm, modeByMAC)
			Expect(result).To(ContainSubstring("00:15:5D:01:02:03"))
			Expect(result).NotTo(ContainSubstring("00:15:5D:01:02:04"))
		})

		It("should skip NICs with mode 'dhcp'", func() {
			b := createBuilder()
			vm := &model.VM{
				GuestOS: "Windows Server 2019",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "172.29.3.193", Origin: hyperv.OriginManual, PrefixLength: 16, Gateway: "172.29.3.1"},
				},
			}
			modeByMAC := map[string]string{
				"00:15:5D:01:02:03": "dhcp",
			}
			result := b.mapMacStaticIps(vm, modeByMAC)
			Expect(result).To(BeEmpty())
		})

		It("should include NICs not in modeByMAC (backward compat)", func() {
			b := createBuilder()
			vm := &model.VM{
				GuestOS: "Windows Server 2019",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "172.29.3.193", Origin: hyperv.OriginManual, PrefixLength: 16, Gateway: "172.29.3.1"},
				},
			}
			modeByMAC := map[string]string{}
			result := b.mapMacStaticIps(vm, modeByMAC)
			Expect(result).To(ContainSubstring("00:15:5D:01:02:03"))
		})

		It("should include all NICs when modeByMAC is nil", func() {
			b := createBuilder()
			vm := &model.VM{
				GuestOS: "Windows Server 2019",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "172.29.3.193", Origin: hyperv.OriginManual, PrefixLength: 16, Gateway: "172.29.3.1"},
					{MAC: "00:15:5D:01:02:04", IP: "172.29.3.194", Origin: hyperv.OriginManual, PrefixLength: 16, Gateway: "172.29.3.1"},
				},
			}
			result := b.mapMacStaticIps(vm, nil)
			Expect(result).To(ContainSubstring("00:15:5D:01:02:03"))
			Expect(result).To(ContainSubstring("00:15:5D:01:02:04"))
		})

		It("should preserve only marked NICs in a mixed-mode map", func() {
			b := createBuilder()
			vm := &model.VM{
				GuestOS: "Windows Server 2019",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "172.29.3.193", Origin: hyperv.OriginManual, PrefixLength: 16, Gateway: "172.29.3.1"},
					{MAC: "00:15:5D:01:02:04", IP: "172.29.3.194", Origin: hyperv.OriginManual, PrefixLength: 16, Gateway: "172.29.3.1"},
					{MAC: "00:15:5D:01:02:05", IP: "172.29.3.195", Origin: hyperv.OriginManual, PrefixLength: 16, Gateway: "172.29.3.1"},
				},
			}
			modeByMAC := map[string]string{
				"00:15:5D:01:02:03": "preserve",
				"00:15:5D:01:02:04": "dhcp",
				"00:15:5D:01:02:05": "none",
			}
			result := b.mapMacStaticIps(vm, modeByMAC)
			Expect(result).To(ContainSubstring("00:15:5D:01:02:03"))
			Expect(result).NotTo(ContainSubstring("00:15:5D:01:02:04"))
			Expect(result).NotTo(ContainSubstring("00:15:5D:01:02:05"))
		})
	})

})

func createBuilder() *Builder {
	return &Builder{
		Context: &plancontext.Context{
			Plan: &v1beta1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-plan",
					Namespace: "test",
				},
			},
			Log: builderLog,
		},
	}
}
