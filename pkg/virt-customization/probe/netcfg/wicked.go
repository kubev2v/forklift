package netcfg

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// Wicked parses wicked lease data from /var/lib/wicked/.
// filesSection contains the `ls` output (filenames, one per line).
// xmlSection contains the concatenated XML from `glob cat lease-*`.
// Interface names and address families are extracted from filenames
// (e.g. "lease-eth0-dhcp-ipv4.xml" → name="eth0", family="ipv4").
// IP addresses are extracted from <address> elements in the XML.
func Wicked(filesSection, xmlSection string) ([]api.InterfaceInfo, error) {
	leases := parseLeaseFilenames(filesSection)
	addresses, err := parseLeaseXML(xmlSection)
	if err != nil {
		return nil, fmt.Errorf("Wicked parser: %w", err)
	}

	var interfaces []api.InterfaceInfo
	for i, lease := range leases {
		if lease.name == "lo" {
			continue
		}
		iface := api.InterfaceInfo{
			Name:   lease.name,
			DHCP:   true,
			Source: "wicked",
		}
		if i < len(addresses) && addresses[i] != "" {
			if lease.family == "ipv6" {
				iface.IPv6 = []string{addresses[i]}
			} else {
				iface.IPv4 = []string{addresses[i]}
			}
		}
		interfaces = append(interfaces, iface)
	}
	return interfaces, nil
}

type leaseFile struct {
	name   string // interface name (e.g. "eth0")
	family string // "ipv4" or "ipv6"
}

// parseLeaseFilenames extracts interface names and address families from
// wicked lease filenames. Format: lease-{interface}-dhcp-{family}.xml
func parseLeaseFilenames(section string) []leaseFile {
	var leases []leaseFile
	for _, line := range strings.Split(strings.TrimSpace(section), "\n") {
		fname := strings.TrimSpace(line)
		if fname == "" || !strings.HasPrefix(fname, "lease-") {
			continue
		}
		fname = strings.TrimPrefix(fname, "lease-")
		idx := strings.Index(fname, "-dhcp-")
		if idx <= 0 {
			continue
		}
		name := fname[:idx]
		rest := fname[idx+len("-dhcp-"):]
		family := strings.TrimSuffix(rest, ".xml")
		leases = append(leases, leaseFile{name: name, family: family})
	}
	return leases
}

// parseLeaseXML extracts the <address> from each <lease> document in
// a concatenated XML stream. Multiple <lease> root elements are handled
// by decoding sequentially.
func parseLeaseXML(section string) ([]string, error) {
	var addresses []string
	decoder := xml.NewDecoder(strings.NewReader(section))
	for {
		addr, err := decodeOneLease(decoder)
		if err == io.EOF {
			break
		}
		if err != nil {
			return addresses, fmt.Errorf("decoding lease XML: %w", err)
		}
		addresses = append(addresses, addr)
	}
	return addresses, nil
}

// decodeOneLease advances the decoder past the next <lease> element and
// extracts the first <address> found inside it.
func decodeOneLease(decoder *xml.Decoder) (string, error) {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return "", err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "lease" {
			return extractAddressFromLease(decoder)
		}
	}
}

// extractAddressFromLease reads tokens inside a <lease> element until
// it finds <address> or hits </lease>.
func extractAddressFromLease(decoder *xml.Decoder) (string, error) {
	var address string
	depth := 1
	for depth > 0 {
		tok, err := decoder.Token()
		if err == io.EOF {
			return address, fmt.Errorf("unexpected EOF inside <lease> element (truncated XML?)")
		}
		if err != nil {
			return address, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "address" {
				var content string
				if err := decoder.DecodeElement(&content, &t); err != nil {
					return address, fmt.Errorf("decoding <address> element: %w", err)
				}
				address = strings.SplitN(content, "/", 2)[0]
				depth--
			}
		case xml.EndElement:
			depth--
		}
	}
	return address, nil
}
