package powermax

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/storage/resolver"
)

// PowermaxImporter implements CsiImportPlugin for Dell PowerMax via the Unisphere REST API.
// Mirrors xcopy's PowermaxClonner naming and credential-in-struct pattern.
type PowermaxImporter struct {
	baseURL     string // e.g. "https://10.0.0.1:8443"
	user        string
	pass        string
	symmetrixID string
	client      *http.Client
}

func NewPowermaxImporter(host, user, pass, symmetrixID string, skipSSL bool) (*PowermaxImporter, error) {
	host = resolver.EnsureScheme(host)
	parsedURL, err := url.Parse(host)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("STORAGE_HOSTNAME must be a full URL with scheme (e.g. https://host:8443), got %q", host)
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
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: skipSSL}, //nolint:gosec
			},
		},
	}, nil
}

const (
	annotationKey = "csi.dell.com/importVolAsClone"
	apiVersion    = "100"
)

// Resolve returns the PVC annotations the Dell CSI PowerMax driver needs to import the source array volume.
func (p *PowermaxImporter) Resolve(backing *resolver.DiskBacking) (map[string]string, error) {
	if backing == nil {
		return nil, fmt.Errorf("nil disk backing")
	}
	if diskType := resolver.DetectDiskType(backing); diskType != resolver.DiskTypeRDM {
		return nil, fmt.Errorf("PowerMax CSI import supports only RDM disks, got %s", diskType)
	}
	return p.resolveRDM(backing.DeviceName)
}

// resolveRDM maps a vSphere RDM device name to a PowerMax volume ID via its WWN.
// DeviceName can be "naa.<hex>" or VML format "vml.<hex>". For VML, the WWN is embedded
// at hex positions 10–42 (NAA type 6 = IEEE Registered Extended). Unisphere stores the
// WWN as uppercase hex without the "naa." prefix.
func (p *PowermaxImporter) resolveRDM(deviceName string) (map[string]string, error) {
	wwn, err := extractWWN(deviceName)
	if err != nil {
		return nil, fmt.Errorf("PowerMax RDM resolution failed (DeviceName: %s): %w", deviceName, err)
	}
	volumeID, err := p.volumeIDByWWN(wwn)
	if err != nil {
		return nil, fmt.Errorf("PowerMax RDM resolution failed (DeviceName: %s): %w", deviceName, err)
	}
	return map[string]string{annotationKey: volumeID}, nil
}

// extractWWN returns the uppercase PowerMax WWN from either NAA or VML vSphere device name format.
func extractWWN(deviceName string) (string, error) {
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
	default:
		return strings.ToUpper(deviceName), nil
	}
}

// volumeIDByWWN queries GET /univmax/restapi/<version>/sloprovisioning/symmetrix/<symID>/volume?wwn=<wwn>
// and returns the first match's volume ID.
func (p *PowermaxImporter) volumeIDByWWN(wwn string) (string, error) {
	queryURL := fmt.Sprintf("%s/univmax/restapi/%s/sloprovisioning/symmetrix/%s/volume?wwn=%s",
		p.baseURL, apiVersion, url.PathEscape(p.symmetrixID), url.QueryEscape(wwn))
	req, err := http.NewRequest(http.MethodGet, queryURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build Unisphere request: %w", err)
	}
	req.SetBasicAuth(p.user, p.pass)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Unisphere volumes request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Unisphere response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Unisphere API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Count      int `json:"count"`
		ResultList struct {
			Result []struct {
				VolumeID string `json:"volumeId"`
			} `json:"result"`
		} `json:"resultList"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse Unisphere response: %w", err)
	}
	if result.Count == 0 || len(result.ResultList.Result) == 0 {
		return "", fmt.Errorf("no PowerMax volume found for wwn=%s on array %s", wwn, p.symmetrixID)
	}
	return result.ResultList.Result[0].VolumeID, nil
}
