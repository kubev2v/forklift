package netcfg

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// Interfaces parses Debian /etc/network/interfaces format.
// Reads iface stanzas and address lines, skipping loopback.
// Distinguishes inet (IPv4) from inet6 (IPv6) stanzas.
func Interfaces(section string) ([]api.InterfaceInfo, error) {
	var interfaces []api.InterfaceInfo
	var current api.InterfaceInfo
	current.Source = "interfaces"
	isV6 := false
	hasData := false

	scanner := bufio.NewScanner(strings.NewReader(section))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		// iface <name> inet|inet6 <method>
		if len(fields) >= 4 && fields[0] == "iface" {
			if fields[1] == "lo" {
				if hasData {
					interfaces = append(interfaces, current)
				}
				hasData = false
				current = api.InterfaceInfo{Source: "interfaces"}
				continue
			}
			if hasData {
				interfaces = append(interfaces, current)
				current = api.InterfaceInfo{Source: "interfaces"}
			}
			current.Name = fields[1]
			isV6 = fields[2] == "inet6"
			current.DHCP = fields[3] == "dhcp"
			hasData = true
		} else if len(fields) >= 2 && fields[0] == "address" && hasData {
			ip := strings.SplitN(fields[1], "/", 2)[0]
			if isV6 {
				current.IPv6 = append(current.IPv6, ip)
			} else {
				current.IPv4 = append(current.IPv4, ip)
			}
		}
	}
	if hasData {
		interfaces = append(interfaces, current)
	}
	if err := scanner.Err(); err != nil {
		return interfaces, fmt.Errorf("Interfaces parser: scanner error: %w", err)
	}
	return interfaces, nil
}
