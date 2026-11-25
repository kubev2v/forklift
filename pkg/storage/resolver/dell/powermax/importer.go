package powermax

import (
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/storage/resolver"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/klog/v2"
)

// annotationKey is the PVC annotation the csi-powermax driver reads (import.storage.dell.com/volumeName).
// Its value is "<symmetrixID>/<deviceID>".
const annotationKey = "import.storage.dell.com/volumeName"

const (
	apiVersion = "100"
	// emcNAAPrefix is the NAA type-6 prefix of every Symmetrix/PowerMax device.
	emcNAAPrefix = "60000970"
)

// PowermaxImporter implements CsiImportPlugin for Dell PowerMax via Unisphere.
type PowermaxImporter struct {
	baseURL     string
	user        string
	pass        string
	symmetrixID string
	client      *http.Client
}

// NewPowermaxImporter validates the endpoint. Only https is accepted; skipSSL only skips certificate checks.
func NewPowermaxImporter(host, user, pass, symmetrixID string, skipSSL bool) (*PowermaxImporter, error) {
	host = resolver.EnsureScheme(host)
	parsedURL, err := url.Parse(host)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return nil, fmt.Errorf("STORAGE_HOSTNAME must be an https URL (e.g. https://host:8443), got %q", host)
	}
	if symmetrixID == "" {
		return nil, fmt.Errorf("storage secret missing required key: POWERMAX_SYMMETRIX_ID")
	}
	return &PowermaxImporter{
		baseURL:     strings.TrimRight(host, "/"),
		user:        user,
		pass:        pass,
		symmetrixID: symmetrixID,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: skipSSL}}, //nolint:gosec
		},
	}, nil
}

// Resolve returns the PVC annotation for a physical-mode RDM. It returns (nil, nil) for VVol, non-PowerMax
// devices and virtual-mode RDMs without snapshots so the disk falls through to other migration paths.
func (p *PowermaxImporter) Resolve(backing *resolver.DiskBacking) (map[string]string, error) {
	if backing == nil {
		return nil, fmt.Errorf("nil disk backing")
	}
	switch resolver.DetectDiskType(backing) {
	case resolver.DiskTypeVVol:
		return nil, nil //nolint:nilnil
	case resolver.DiskTypeRDM:
	default:
		return nil, fmt.Errorf("PowerMax CSI import supports only RDM disks")
	}
	if backing.CompatibilityMode == string(types.VirtualDiskCompatibilityModeVirtualMode) {
		if backing.VMHasSnapshots {
			return nil, fmt.Errorf("virtual-mode RDM with VM snapshots cannot be imported: snapshot data resides on a VMFS datastore, not on the LUN")
		}
		klog.InfoS("PowerMax CSI import: virtual-mode RDM is not supported, falling through", "device", backing.DeviceName)
		return nil, nil //nolint:nilnil
	}
	device := backing.DeviceName
	if device == "" {
		device = backing.LunUuid
	}
	wwn, err := extractWWN(device)
	if err != nil {
		return nil, fmt.Errorf("PowerMax RDM resolution failed (device: %s): %w", device, err)
	}
	if err := checkNAA(wwn); err != nil {
		return nil, fmt.Errorf("PowerMax RDM resolution failed (device: %s): %w", device, err)
	}
	// ESXi reports the effective (spoofed) WWN for migrated devices, so match either WWN.
	ids, err := p.deviceIDsByWWN("wwn", wwn)
	if err == nil && len(ids) == 0 {
		ids, err = p.deviceIDsByWWN("effective_wwn", wwn)
	}
	if err != nil {
		return nil, fmt.Errorf("PowerMax RDM resolution failed (device: %s): %w", device, err)
	}
	switch {
	case len(ids) == 0 && !strings.HasPrefix(wwn, emcNAAPrefix):
		return nil, nil //nolint:nilnil
	case len(ids) != 1:
		return nil, fmt.Errorf("PowerMax RDM resolution failed (device: %s): expected 1 volume for WWN %s on array %s, found %d", device, wwn, p.symmetrixID, len(ids))
	}
	return map[string]string{annotationKey: p.symmetrixID + "/" + ids[0]}, nil
}

// checkNAA validates a 32-hex NAA and, for a PowerMax device, its layout: vendor prefix, 12-digit Symmetrix ID,
// 'S' marker and a device ID encoded as ASCII hex.
func checkNAA(wwn string) error {
	if _, err := hex.DecodeString(wwn); err != nil || len(wwn) != 32 {
		return fmt.Errorf("malformed NAA identifier %q", wwn)
	}
	if !strings.HasPrefix(wwn, emcNAAPrefix) {
		return nil
	}
	if wwn[20:22] != "53" {
		return fmt.Errorf("malformed NAA identifier %q: unexpected marker %s at hex characters 20 to 22, expected 53 ('S')", wwn, wwn[20:22])
	}
	if id, _ := hex.DecodeString(wwn[22:]); strings.Trim(string(id), "0123456789ABCDEFabcdef") != "" {
		return fmt.Errorf("malformed NAA identifier %q: device ID is not ASCII hex", wwn)
	}
	return nil
}

// extractWWN returns the uppercase PowerMax WWN from an NAA or VML device name, with or without a
// /vmfs/devices/disks/ path, or from a bare LUN UUID (the VML name without its "vml." prefix).
func extractWWN(deviceName string) (string, error) {
	deviceName = deviceName[strings.LastIndex(deviceName, "/")+1:]
	switch {
	case strings.HasPrefix(deviceName, "naa."):
		return strings.ToUpper(strings.TrimPrefix(deviceName, "naa.")), nil
	case strings.HasPrefix(deviceName, "vml."):
		hex := strings.TrimPrefix(deviceName, "vml.")
		if len(hex) < 42 {
			return "", fmt.Errorf("VML string too short to contain NAA: %s", deviceName)
		}
		naa := hex[10:42]
		if !strings.HasPrefix(naa, "6") {
			return "", fmt.Errorf("VML does not contain NAA type 6 identifier: %s", deviceName)
		}
		return strings.ToUpper(naa), nil
	case len(deviceName) >= 42 && strings.HasPrefix(deviceName, "02"):
		return extractWWN("vml." + deviceName)
	default:
		return strings.ToUpper(deviceName), nil
	}
}

// deviceIDsByWWN returns the IDs of the devices whose wwn or effective_wwn (field) equals the WWN. Several
// matches are an error for the caller: guessing could import the wrong source device.
func (p *PowermaxImporter) deviceIDsByWWN(field, wwn string) ([]string, error) {
	var out struct {
		ResultList struct {
			Result []struct {
				VolumeID string `json:"volumeId"`
			} `json:"result"`
		} `json:"resultList"`
	}
	if err := p.call(http.MethodGet, "/volume?"+field+"="+url.QueryEscape(wwn), &out); err != nil {
		return nil, err
	}
	var ids []string
	for _, r := range out.ResultList.Result {
		ids = append(ids, r.VolumeID)
	}
	return ids, nil
}

// call performs one Unisphere sloprovisioning request and decodes the JSON response into out.
func (p *PowermaxImporter) call(method, path string, out interface{}) error {
	reqURL := p.baseURL + "/univmax/restapi/" + apiVersion + "/sloprovisioning/symmetrix/" + url.PathEscape(p.symmetrixID) + path
	req, err := http.NewRequest(method, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to build Unisphere request: %w", err)
	}
	req.SetBasicAuth(p.user, p.pass)
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("unisphere request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read Unisphere response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unisphere API returned %d: %s", resp.StatusCode, string(data))
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("failed to parse Unisphere response: %w", err)
		}
	}
	return nil
}
