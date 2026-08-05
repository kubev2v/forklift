package netcfg

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// Dhclient parses dhclient lease files (concatenated contents of all
// dhclient-* files from /var/lib/dhclient/ and /var/lib/NetworkManager/).
//
// Lease blocks have the form:
//
//	lease {
//	  interface "eth0";
//	  fixed-address 192.168.122.82;
//	  ...
//	  expire 3 2025/04/30 17:42:30;
//	}
//
// When multiple blocks share the same interface, the one with the latest
// expiration is kept. Each unique (interface, latest-IP) pair becomes one
// InterfaceInfo entry.
func Dhclient(section string) ([]api.InterfaceInfo, error) {
	blocks := parseDhclientBlocks(section)

	type best struct {
		iface  string
		ip     string
		expire time.Time
	}
	latest := map[string]best{}

	for _, b := range blocks {
		if b.iface == "" || b.ip == "" {
			continue
		}
		prev, exists := latest[b.iface]
		if !exists || b.expire.After(prev.expire) {
			latest[b.iface] = best{iface: b.iface, ip: b.ip, expire: b.expire}
		}
	}

	var interfaces []api.InterfaceInfo
	for _, entry := range latest {
		iface := api.InterfaceInfo{
			Name:   entry.iface,
			DHCP:   true,
			Source: "dhclient",
		}
		if strings.Contains(entry.ip, ":") {
			iface.IPv6 = []string{entry.ip}
		} else {
			iface.IPv4 = []string{entry.ip}
		}
		interfaces = append(interfaces, iface)
	}
	return interfaces, nil
}

type dhclientBlock struct {
	iface  string
	ip     string
	expire time.Time
}

// parseDhclientBlocks splits concatenated dhclient lease content into blocks
// and extracts interface, fixed-address, and expire from each.
func parseDhclientBlocks(section string) []dhclientBlock {
	var blocks []dhclientBlock
	var current dhclientBlock
	inBlock := false

	scanner := bufio.NewScanner(strings.NewReader(section))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimSuffix(line, ";")
		line = strings.TrimSpace(line)

		if line == "lease {" {
			inBlock = true
			current = dhclientBlock{}
			continue
		}
		if line == "}" && inBlock {
			blocks = append(blocks, current)
			inBlock = false
			continue
		}
		if !inBlock {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "interface":
			current.iface = stripQuotes(fields[1])
		case "fixed-address":
			current.ip = fields[1]
		case "expire":
			current.expire = parseDhclientExpire(fields[1:])
		}
	}
	return blocks
}

// stripQuotes removes surrounding double quotes from a string.
func stripQuotes(s string) string {
	return strings.Trim(s, `"`)
}

// parseDhclientExpire parses the expire fields: <dayofweek> <YYYY/MM/DD> <HH:MM:SS>
func parseDhclientExpire(fields []string) time.Time {
	if len(fields) < 3 {
		return time.Time{}
	}
	dateStr := fmt.Sprintf("%s %s", fields[1], fields[2])
	t, err := time.Parse("2006/01/02 15:04:05", dateStr)
	if err != nil {
		return time.Time{}
	}
	return t
}
