package probe

import (
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
)

// Detection output order (10 checks):
// windows, os-release, ifcfg, ifcfg-suse, network-manager, netplan, ifquery, interfaces-d, wicked-etc, wicked-var

func TestParseDetection_Windows(t *testing.T) {
	t.Parallel()
	output := "true\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse"
	guest := &api.GuestInfo{}
	parseDetection(output, guest)
	if guest.OS.Family != api.OSFamilyWindows {
		t.Errorf("expected Windows, got %s", guest.OS.Family)
	}
}

func TestParseDetection_Linux(t *testing.T) {
	t.Parallel()
	output := "false\ntrue\ntrue\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse"
	guest := &api.GuestInfo{}
	parseDetection(output, guest)
	if guest.OS.Family != api.OSFamilyLinux {
		t.Errorf("expected Linux, got %s", guest.OS.Family)
	}
	if !guest.UsesIfcfg {
		t.Error("expected UsesIfcfg to be true")
	}
	if guest.UsesNetworkManager {
		t.Error("expected UsesNetworkManager to be false")
	}
}

func TestParseDetection_IfcfgSuse(t *testing.T) {
	t.Parallel()
	output := "false\ntrue\nfalse\ntrue\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse"
	guest := &api.GuestInfo{}
	parseDetection(output, guest)
	if !guest.UsesIfcfgSuse {
		t.Error("expected UsesIfcfgSuse to be true")
	}
	if guest.UsesIfcfg {
		t.Error("expected UsesIfcfg to be false")
	}
}

func TestParseDetection_NetworkManager(t *testing.T) {
	t.Parallel()
	output := "false\ntrue\nfalse\nfalse\ntrue\nfalse\nfalse\nfalse\nfalse\nfalse"
	guest := &api.GuestInfo{}
	parseDetection(output, guest)
	if !guest.UsesNetworkManager {
		t.Error("expected UsesNetworkManager to be true")
	}
	if guest.UsesIfcfg {
		t.Error("expected UsesIfcfg to be false")
	}
}

func TestParseDetection_Netplan(t *testing.T) {
	t.Parallel()
	output := "false\ntrue\nfalse\nfalse\nfalse\ntrue\nfalse\nfalse\nfalse\nfalse"
	guest := &api.GuestInfo{}
	parseDetection(output, guest)
	if !guest.UsesNetplan {
		t.Error("expected UsesNetplan to be true")
	}
}

func TestParseDetection_Ifquery(t *testing.T) {
	t.Parallel()
	output := "false\ntrue\nfalse\nfalse\nfalse\nfalse\ntrue\nfalse\nfalse\nfalse"
	guest := &api.GuestInfo{}
	parseDetection(output, guest)
	if !guest.UsesIfquery {
		t.Error("expected UsesIfquery to be true")
	}
}

func TestParseDetection_InterfacesD(t *testing.T) {
	t.Parallel()
	output := "false\ntrue\nfalse\nfalse\nfalse\nfalse\nfalse\ntrue\nfalse\nfalse"
	guest := &api.GuestInfo{}
	parseDetection(output, guest)
	if !guest.UsesInterfacesD {
		t.Error("expected UsesInterfacesD to be true")
	}
}

func TestParseDetection_Wicked_EtcWicked(t *testing.T) {
	t.Parallel()
	output := "false\ntrue\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse\ntrue\nfalse"
	guest := &api.GuestInfo{}
	parseDetection(output, guest)
	if !guest.UsesWicked {
		t.Error("expected UsesWicked to be true")
	}
}

func TestParseDetection_Wicked_VarLibWicked(t *testing.T) {
	t.Parallel()
	output := "false\ntrue\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse\ntrue"
	guest := &api.GuestInfo{}
	parseDetection(output, guest)
	if !guest.UsesWicked {
		t.Error("expected UsesWicked to be true")
	}
}

func TestParseDetection_Unknown(t *testing.T) {
	t.Parallel()
	output := "false\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse\nfalse"
	guest := &api.GuestInfo{}
	parseDetection(output, guest)
	if guest.OS.Family != api.OSFamilyUnknown {
		t.Errorf("expected Unknown, got %s", guest.OS.Family)
	}
}

func TestParseOsRelease_RHEL(t *testing.T) {
	t.Parallel()
	output := "NAME=\"Red Hat Enterprise Linux\"\nID=rhel\nVERSION_ID=\"9.2\"\n===IFCFG_START===\n"
	guest := &api.GuestInfo{}
	if err := parseOsRelease(output, guest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if guest.OS.Distro != "rhel" {
		t.Errorf("expected rhel, got %s", guest.OS.Distro)
	}
	if guest.OS.Version != "9.2" {
		t.Errorf("expected 9.2, got %s", guest.OS.Version)
	}
}

func TestParseOsRelease_Ubuntu(t *testing.T) {
	t.Parallel()
	output := "NAME=\"Ubuntu\"\nID=ubuntu\nVERSION_ID=\"22.04\"\n"
	guest := &api.GuestInfo{}
	if err := parseOsRelease(output, guest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if guest.OS.Distro != "ubuntu" {
		t.Errorf("expected ubuntu, got %s", guest.OS.Distro)
	}
	if guest.OS.Version != "22.04" {
		t.Errorf("expected 22.04, got %s", guest.OS.Version)
	}
}

func TestBuildExtractionScript_IfcfgOnly(t *testing.T) {
	t.Parallel()
	guest := &api.GuestInfo{
		OS:        api.GuestOS{Family: api.OSFamilyLinux},
		UsesIfcfg: true,
	}
	script := buildExtractionScript(guest)
	if !strings.Contains(script, "===IFCFG_START===") {
		t.Error("expected ===IFCFG_START===")
	}
	if strings.Contains(script, "===NM_START===") {
		t.Error("did not expect ===NM_START===")
	}
}

func TestBuildExtractionScript_Windows(t *testing.T) {
	t.Parallel()
	guest := &api.GuestInfo{
		OS: api.GuestOS{Family: api.OSFamilyWindows},
	}
	script := buildExtractionScript(guest)
	if script != "" {
		t.Error("expected empty script for Windows")
	}
}

func TestBuildExtractionScript_MultipleStacks(t *testing.T) {
	t.Parallel()
	guest := &api.GuestInfo{
		OS:                 api.GuestOS{Family: api.OSFamilyLinux},
		UsesIfcfg:          true,
		UsesNetworkManager: true,
	}
	script := buildExtractionScript(guest)
	if !strings.Contains(script, "===IFCFG_START===") {
		t.Error("expected ===IFCFG_START===")
	}
	if !strings.Contains(script, "===NM_START===") {
		t.Error("expected ===NM_START===")
	}
}

func TestExtractSection_Present(t *testing.T) {
	t.Parallel()
	output := "before\n===START===\ncontent line 1\ncontent line 2\n===END===\nafter"
	section := extractSection(output, "===START===", "===END===")
	if section != "content line 1\ncontent line 2\n" {
		t.Errorf("unexpected section: %q", section)
	}
}

func TestExtractSection_Missing(t *testing.T) {
	t.Parallel()
	output := "no markers here"
	section := extractSection(output, "===START===", "===END===")
	if section != "" {
		t.Errorf("expected empty, got %q", section)
	}
}
