package netcfg

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// Ifcfg parses RHEL/CentOS/SUSE ifcfg-style configuration output.
// Supports both bare IPADDR and numbered IPADDR0, IPADDR1, etc.
// IPv6 addresses are extracted from IPV6ADDR and IPV6ADDR_SECONDARIES.
// Entries in the concatenated input are split on blank lines or on a new
// DEVICE= line (whichever comes first), so the parser handles both
// blank-line-separated and directly concatenated ifcfg files.
func Ifcfg(section string) ([]api.InterfaceInfo, error) {
	var interfaces []api.InterfaceInfo
	var current api.InterfaceInfo
	current.Source = "ifcfg"

	scanner := bufio.NewScanner(strings.NewReader(section))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if current.Name != "" || current.HasIPs() {
				interfaces = append(interfaces, current)
				current = api.InterfaceInfo{Source: "ifcfg"}
			}
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := kv[0]
		val := strings.Trim(kv[1], "\"'")
		switch key {
		case "DEVICE":
			if current.Name != "" || current.HasIPs() {
				interfaces = append(interfaces, current)
				current = api.InterfaceInfo{Source: "ifcfg"}
			}
			current.Name = val
		case "HWADDR":
			current.MAC = strings.ToLower(val)
		case "NAME":
			if current.Name == "" {
				current.Name = val
			}
		case "BOOTPROTO":
			current.DHCP = strings.EqualFold(val, "dhcp")
		case "IPV6ADDR":
			if ip := stripPrefix(val); ip != "" {
				current.IPv6 = append(current.IPv6, ip)
			}
		case "IPV6ADDR_SECONDARIES":
			for _, addr := range strings.Fields(val) {
				if ip := stripPrefix(addr); ip != "" {
					current.IPv6 = append(current.IPv6, ip)
				}
			}
		default:
			if strings.HasPrefix(key, "IPADDR") && val != "" {
				current.IPv4 = append(current.IPv4, val)
			}
		}
	}
	if current.Name != "" || current.HasIPs() {
		interfaces = append(interfaces, current)
	}
	if err := scanner.Err(); err != nil {
		return interfaces, fmt.Errorf("Ifcfg parser: scanner error: %w", err)
	}
	return interfaces, nil
}

// stripPrefix removes a CIDR prefix length ("/64") if present.
func stripPrefix(addr string) string {
	return strings.SplitN(addr, "/", 2)[0]
}
