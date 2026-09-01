package hyperv

import (
	"fmt"
	"testing"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
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

func TestMapMemory(t *testing.T) {
	r := &Builder{}
	vm := &model.VM{}
	vm.MemoryMB = 2048
	object := &cnv.VirtualMachineSpec{
		Template: &cnv.VirtualMachineInstanceTemplateSpec{},
	}

	r.mapMemory(vm, object, false)

	if object.Template.Spec.Domain.Memory == nil || object.Template.Spec.Domain.Memory.Guest == nil {
		t.Fatalf("expected Memory.Guest to be set, got nil")
	}
	expected := int64(2048) * 1024 * 1024
	actual := object.Template.Spec.Domain.Memory.Guest.Value()
	if actual != expected {
		t.Errorf("Memory.Guest = %d, want %d", actual, expected)
	}
}

func TestMapMemoryUsesInstanceType(t *testing.T) {
	r := &Builder{}
	vm := &model.VM{}
	vm.MemoryMB = 2048
	object := &cnv.VirtualMachineSpec{
		Template: &cnv.VirtualMachineInstanceTemplateSpec{},
	}

	r.mapMemory(vm, object, true)

	if object.Template.Spec.Domain.Memory != nil {
		t.Errorf("expected Memory to be nil when usesInstanceType is true, got %+v", object.Template.Spec.Domain.Memory)
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

func TestMapHypervGuestOS(t *testing.T) {
	tests := []struct {
		name     string
		guestOS  string
		expected string
	}{
		// Windows variants
		{name: "Win Server 2022", guestOS: "Microsoft Windows Server 2022 Standard", expected: "win2k22"},
		{name: "Win Server 2019", guestOS: "Microsoft Windows Server 2019 Datacenter", expected: "win2k19"},
		{name: "Win Server 2016", guestOS: "Microsoft Windows Server 2016 Standard", expected: "win2k16"},
		{name: "Win Server 2012 R2", guestOS: "Microsoft Windows Server 2012 R2 Datacenter", expected: "win2k12r2"},
		{name: "Win Server 2012 (non-R2)", guestOS: "Microsoft Windows Server 2012 Datacenter", expected: "win2k12r2"},
		{name: "Windows 11", guestOS: "Microsoft Windows 11 Enterprise", expected: "win11"},
		{name: "Windows 10", guestOS: "Microsoft Windows 10 Pro", expected: "win10"},
		{name: "Generic Windows", guestOS: "Microsoft Windows", expected: "win10"},

		// RHEL variants - major version detection
		{name: "RHEL 9.2", guestOS: "Red Hat Enterprise Linux 9.2 (Plow)", expected: "rhel9.4"},
		{name: "RHEL 8.4", guestOS: "Red Hat Enterprise Linux 8.4 (Ootpa)", expected: defaultTemplateOS},
		{name: "RHEL 7.9", guestOS: "Red Hat Enterprise Linux 7.9 (Maipo)", expected: "rhel7.7"},
		{name: "RHEL 7 no minor", guestOS: "Red Hat Enterprise Linux 7", expected: "rhel7.7"},
		{name: "RHEL 8 no minor", guestOS: "Red Hat Enterprise Linux 8", expected: defaultTemplateOS},
		{name: "RHEL 9 no minor", guestOS: "Red Hat Enterprise Linux 9", expected: "rhel9.4"},
		{name: "RHEL unknown version", guestOS: "Red Hat Enterprise Linux", expected: defaultTemplateOS},

		// Version boundary: minor version must not confuse major detection
		{name: "RHEL 7.9 not misidentified as 9", guestOS: "Red Hat Enterprise Linux 7.9", expected: "rhel7.7"},
		{name: "RHEL 8.9 not misidentified as 9", guestOS: "Red Hat Enterprise Linux 8.9", expected: defaultTemplateOS},
		{name: "CentOS 7.9 not misidentified as 9", guestOS: "CentOS Linux 7.9.2009 (Core)", expected: "centos7.0"},

		// CentOS variants
		{name: "CentOS Stream 9", guestOS: "CentOS Stream 9", expected: "centos-stream9"},
		{name: "CentOS 8.5", guestOS: "CentOS Linux 8.5.2111", expected: "centos8"},
		{name: "CentOS 7.6", guestOS: "CentOS Linux 7.6.1810 (Core)", expected: "centos7.0"},
		{name: "CentOS generic", guestOS: "CentOS Linux", expected: "centos7.0"},

		// Other Linux
		{name: "Ubuntu", guestOS: "Ubuntu 22.04 LTS", expected: "ubuntu18.04"},
		{name: "Debian", guestOS: "Debian GNU/Linux 11", expected: "debian10"},
		{name: "Fedora", guestOS: "Fedora Linux 38", expected: "fedora31"},
		{name: "SUSE", guestOS: "SUSE Linux Enterprise Server 15", expected: "opensuse15.0"},
		{name: "SLES", guestOS: "SLES 12 SP5", expected: "opensuse15.0"},
		{name: "Generic Linux", guestOS: "Some Linux Distribution", expected: defaultTemplateOS},

		// Fallback
		{name: "Unknown OS", guestOS: "FreeBSD 13", expected: ""},
		{name: "Empty string", guestOS: "", expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := mapHypervGuestOS(tc.guestOS)
			if result != tc.expected {
				t.Errorf("mapHypervGuestOS(%q) = %q, want %q", tc.guestOS, result, tc.expected)
			}
		})
	}
}

func TestMapOperatingSystemToTemplate(t *testing.T) {
	tests := []struct {
		name     string
		os       string
		expected string
	}{
		// virt-v2v inspection maps osinfo IDs through osV2VMap to these vSphere-style guest IDs.
		// VMware naming: "Next" suffix = successor (2019srvNext → 2022, 2022srvNext → 2025).
		{name: "Windows 2019", os: "windows2019srv_64Guest", expected: "win2k19"},
		{name: "Windows 2022 (2019srvNext)", os: "windows2019srvNext_64Guest", expected: "win2k22"},
		{name: "Windows 2025 (2022srvNext)", os: "windows2022srvNext_64Guest", expected: "win2k25"},
		{name: "Windows 2016 (9Server)", os: "windows9Server64Guest", expected: "win2k16"},
		{name: "Windows 2012 R2 (8Server)", os: "windows8Server64Guest", expected: "win2k12r2"},
		{name: "Windows 2008 R2 (7Server)", os: "windows7Server64Guest", expected: "win2k8r2"},
		{name: "Windows 10", os: "windows9_64Guest", expected: "win10"},
		{name: "Windows 11", os: "windows11_64Guest", expected: "win11"},
		{name: "RHEL 9", os: "rhel9_64Guest", expected: "rhel9.4"},
		{name: "RHEL 8", os: "rhel8_64Guest", expected: defaultTemplateOS},
		{name: "RHEL 7", os: "rhel7_64Guest", expected: "rhel7.7"},
		{name: "RHEL 10", os: "rhel10_64Guest", expected: "rhel10.0"},
		{name: "CentOS 9", os: "centos9_64Guest", expected: "centos-stream9"},
		{name: "CentOS 8", os: "centos8_64Guest", expected: "centos8"},
		{name: "Ubuntu", os: "ubuntu64Guest", expected: "ubuntu18.04"},
		{name: "Debian", os: "debian10_64Guest", expected: "debian10"},
		{name: "Fedora", os: "fedora64Guest", expected: "fedora31"},
		{name: "SLES", os: "sles15_64Guest", expected: "opensuse15.0"},
		{name: "Generic Linux", os: "genericLinuxGuest", expected: defaultTemplateOS},
		{name: "Empty string", os: "", expected: ""},
		{name: "Unknown OS", os: "otherGuest64", expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := mapOperatingSystemToTemplate(tc.os)
			if result != tc.expected {
				t.Errorf("mapOperatingSystemToTemplate(%q) = %q, want %q", tc.os, result, tc.expected)
			}
		})
	}
}

func TestMapHypervGuestOS_TemplateLabels(t *testing.T) {
	guestOS := "Red Hat Enterprise Linux 8.4 (Ootpa)"
	os := mapHypervGuestOS(guestOS)

	expectedLabels := map[string]string{
		"os.template.kubevirt.io/" + os:        "true",
		"workload.template.kubevirt.io/server": "true",
		"flavor.template.kubevirt.io/medium":   "true",
	}

	labels := make(map[string]string)
	labels[fmt.Sprintf(templateOSLabel, os)] = "true"
	labels[templateWorkloadLabel] = "true"
	labels[templateFlavorLabel] = "true"

	if len(labels) != len(expectedLabels) {
		t.Fatalf("expected %d labels, got %d", len(expectedLabels), len(labels))
	}
	for k, v := range expectedLabels {
		if labels[k] != v {
			t.Errorf("label %q = %q, want %q", k, labels[k], v)
		}
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
