package netcfg

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// uuidRe matches a UUID in a lease filename.
// Filename format: prefix-<UUID>-<INTERFACE>.lease
var uuidRe = regexp.MustCompile(`([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})-(.+)\.lease$`)

// NMDhcpLease parses NetworkManager DHCP lease files from /var/lib/NetworkManager/.
// filesSection contains ls output (filenames, one per line).
// leaseSection contains the concatenated lease file contents (each file has
// an optional comment line and an ADDRESS=<ip> line).
// timestampsSection contains the timestamps file (uuid=epoch pairs).
//
// For each lease file the parser extracts the IP from ADDRESS=, the UUID and
// interface name from the filename, and the timestamp from the timestamps file.
// When multiple leases exist for the same interface, the most recent wins.
func NMDhcpLease(filesSection, leaseSection, timestampsSection string) ([]api.InterfaceInfo, error) {
	leaseFiles := parseNMLeaseFilenames(filesSection)
	addresses := parseNMLeaseAddresses(leaseSection)
	timestamps := parseTimestamps(timestampsSection)

	type leaseEntry struct {
		name      string
		ip        string
		timestamp int64
	}

	best := map[string]leaseEntry{}
	for i, lf := range leaseFiles {
		ip := ""
		if i < len(addresses) {
			ip = addresses[i]
		}
		if ip == "" || lf.iface == "" {
			continue
		}

		ts := timestamps[lf.uuid]

		prev, exists := best[lf.iface]
		if !exists || ts > prev.timestamp {
			best[lf.iface] = leaseEntry{name: lf.iface, ip: ip, timestamp: ts}
		}
	}

	var interfaces []api.InterfaceInfo
	for _, entry := range best {
		iface := api.InterfaceInfo{
			Name:   entry.name,
			DHCP:   true,
			Source: "nm-dhcp-lease",
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

type nmLeaseFile struct {
	uuid  string
	iface string
}

// parseNMLeaseFilenames extracts UUID and interface name from NM lease filenames.
func parseNMLeaseFilenames(section string) []nmLeaseFile {
	var files []nmLeaseFile
	for _, line := range strings.Split(strings.TrimSpace(section), "\n") {
		fname := strings.TrimSpace(line)
		if fname == "" || !strings.HasSuffix(fname, ".lease") {
			continue
		}
		matches := uuidRe.FindStringSubmatch(fname)
		if matches == nil {
			continue
		}
		files = append(files, nmLeaseFile{uuid: matches[1], iface: matches[2]})
	}
	return files
}

// parseNMLeaseAddresses extracts ADDRESS= values from concatenated lease contents.
func parseNMLeaseAddresses(section string) []string {
	var addrs []string
	scanner := bufio.NewScanner(strings.NewReader(section))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "ADDRESS=") {
			addrs = append(addrs, strings.TrimPrefix(line, "ADDRESS="))
		}
	}
	return addrs
}

// parseTimestamps parses the NM timestamps file (uuid=epoch pairs).
// Lines like "[timestamps]" headers and blank lines are skipped.
func parseTimestamps(section string) map[string]int64 {
	ts := map[string]int64{}
	scanner := bufio.NewScanner(strings.NewReader(section))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		uuid := strings.TrimSpace(parts[0])
		epoch, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			fmt.Printf("warning: bad timestamp for UUID %s: %v\n", uuid, err)
			continue
		}
		ts[uuid] = epoch
	}
	return ts
}
