package pure

import (
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/pkg/storage/resolver"
)

const (
	annotationKey   = "portworx.io/pure-volume-name"
	flashProviderID = "624a9370"
)

// PureImporter implements CsiImportPlugin for Pure FlashArray via the PX-CSI driver.
// VVol and RDM disk backings are resolved to volume names via the FlashArray REST API
// and returned as the portworx.io/pure-volume-name PVC annotation.
type PureImporter struct {
	client *RestClient
}

func NewPureImporter(host, user, pass string, skipSSL bool) (*PureImporter, error) {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimRight(host, "/")
	if host == "" {
		return nil, fmt.Errorf("STORAGE_HOSTNAME must not be empty")
	}

	c, err := NewRestClient(host, user, pass, "", skipSSL)
	if err != nil {
		return nil, err
	}
	return &PureImporter{client: c}, nil
}

// Resolve returns the PVC annotations the PX-CSI driver needs to import the source array volume.
func (p *PureImporter) Resolve(backing *resolver.DiskBacking) (map[string]string, error) {
	if backing == nil {
		return nil, fmt.Errorf("nil disk backing")
	}
	switch resolver.DetectDiskType(backing) {
	case resolver.DiskTypeVVol:
		return p.resolveVVol(backing.VVolID)
	case resolver.DiskTypeRDM:
		return p.resolveRDM(backing.DeviceName)
	default:
		return nil, fmt.Errorf("pure CSI import does not support VMDK disks")
	}
}

func (p *PureImporter) resolveVVol(vVolID string) (map[string]string, error) {
	name, err := p.client.FindVolumeByVVolID(vVolID)
	if err != nil {
		return nil, fmt.Errorf("VVol resolution failed (VVolID: %s): %w", vVolID, err)
	}
	return map[string]string{annotationKey: name}, nil
}

func (p *PureImporter) resolveRDM(deviceName string) (map[string]string, error) {
	serial, err := extractSerialFromNAA(deviceName)
	if err != nil {
		return nil, fmt.Errorf("RDM resolution failed (DeviceName: %s): %w", deviceName, err)
	}
	name, err := p.client.FindVolumeBySerial(serial)
	if err != nil {
		return nil, fmt.Errorf("RDM resolution failed (DeviceName: %s): %w", deviceName, err)
	}
	return map[string]string{annotationKey: name}, nil
}

// extractSerialFromNAA extracts the uppercase volume serial from a Pure device identifier.
// Supports two formats:
//   - NAA: naa.624a9370<serial>
//   - VML: vml.<hex> — vSphere VML encoding; the Pure OUI 624a9370 is embedded in the hex.
//     The serial is the 24 hex chars immediately following the OUI.
func extractSerialFromNAA(deviceName string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(deviceName))
	oui := strings.ToLower(flashProviderID)

	switch {
	case strings.HasPrefix(lower, "naa."):
		hex := strings.TrimPrefix(lower, "naa.")
		if !strings.HasPrefix(hex, oui) {
			return "", fmt.Errorf("NAA %s does not have Pure OUI prefix %s", deviceName, flashProviderID)
		}
		serial := strings.TrimPrefix(hex, oui)
		if serial == "" {
			return "", fmt.Errorf("could not extract serial from NAA %s", deviceName)
		}
		return strings.ToUpper(serial), nil

	case strings.HasPrefix(lower, "vml."):
		hex := strings.TrimPrefix(lower, "vml.")
		idx := strings.Index(hex, oui)
		if idx < 0 {
			return "", fmt.Errorf("VML %s does not contain Pure OUI prefix %s", deviceName, flashProviderID)
		}
		serial := hex[idx+len(oui):]
		if len(serial) < 24 {
			return "", fmt.Errorf("VML serial too short in %s", deviceName)
		}
		return strings.ToUpper(serial[:24]), nil

	default:
		return "", fmt.Errorf("unrecognised device name format (expected naa. or vml.): %s", deviceName)
	}
}
