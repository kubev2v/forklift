package probe

import (
	"fmt"
	"os"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
)

// detectionChecks defines the ordered guestfish probes and their logical names.
var detectionChecks = []struct {
	name    string
	command string
}{
	{"windows", "is-dir /Windows/System32"},
	{"os-release", "is-file /etc/os-release"},
	{"ifcfg", "is-dir /etc/sysconfig/network-scripts"},
	{"ifcfg-suse", "is-dir /etc/sysconfig/network"},
	{"network-manager", "is-dir /etc/NetworkManager/system-connections"},
	{"netplan", "is-dir /etc/netplan"},
	{"ifquery", "is-file /etc/network/interfaces"},
	{"interfaces-d", "is-dir /etc/network/interfaces.d"},
	{"wicked-etc", "is-dir /etc/wicked"},
	{"wicked-var", "is-dir /var/lib/wicked"},
}

// buildDetectionScript returns a guestfish script that probes for OS type and network stacks.
func buildDetectionScript() string {
	var sb strings.Builder
	for _, check := range detectionChecks {
		sb.WriteString(check.command)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// parseDetection maps guestfish true/false output lines to GuestInfo OS and stack flags.
func parseDetection(output string, guest *api.GuestInfo) {
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) < len(detectionChecks) {
		fmt.Fprintf(os.Stderr, "Warning: truncated guestfish detection output: expected=%d received=%d\n",
			len(detectionChecks), len(lines))
	}

	results := make(map[string]bool, len(detectionChecks))
	for i, check := range detectionChecks {
		if i < len(lines) {
			results[check.name] = strings.TrimSpace(lines[i]) == "true"
		}
	}

	if results["windows"] {
		guest.OS.Family = api.OSFamilyWindows
		return
	}

	if results["os-release"] {
		guest.OS.Family = api.OSFamilyLinux
	} else {
		guest.OS.Family = api.OSFamilyUnknown
	}

	guest.UsesIfcfg = results["ifcfg"]
	guest.UsesIfcfgSuse = results["ifcfg-suse"]
	guest.UsesNetworkManager = results["network-manager"]
	guest.UsesNetplan = results["netplan"]
	guest.UsesIfquery = results["ifquery"]
	guest.UsesInterfacesD = results["interfaces-d"]
	guest.UsesWicked = results["wicked-etc"] || results["wicked-var"]
}
