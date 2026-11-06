package powerstore

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

// PowerstoreImporter implements CsiImportPlugin for Dell PowerStore via its REST API.
// Mirrors xcopy's PowerstoreClonner naming and credential-in-struct pattern.
type PowerstoreImporter struct {
	baseURL string // e.g. "https://10.0.0.1"
	user    string
	pass    string
	client  *http.Client
}

func NewPowerstoreImporter(host, user, pass string, skipSSL bool) (*PowerstoreImporter, error) {
	host = resolver.EnsureScheme(host)
	parsedURL, err := url.Parse(host)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("STORAGE_HOSTNAME must be a full URL with scheme (e.g. https://host), got %q", host)
	}

	return &PowerstoreImporter{
		baseURL: strings.TrimRight(host, "/"),
		user:    user,
		pass:    pass,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: skipSSL}, //nolint:gosec
			},
		},
	}, nil
}

const annotationKey = "csi.dell.com/importVolAsClone"

// Resolve returns the PVC annotations the Dell CSI PowerStore driver needs to import the source array volume.
func (p *PowerstoreImporter) Resolve(backing *resolver.DiskBacking) (map[string]string, error) {
	if backing == nil {
		return nil, fmt.Errorf("nil disk backing")
	}
	if diskType := resolver.DetectDiskType(backing); diskType != resolver.DiskTypeRDM {
		return nil, fmt.Errorf("PowerStore CSI import supports only RDM disks, got %s", diskType)
	}
	return p.resolveRDM(backing.DeviceName)
}

// resolveRDM maps a vSphere RDM device name to a PowerStore volume name via its WWN.
// DeviceName can be "naa.<hex>" or VML format "vml.<hex>". For VML, the WWN is embedded
// at hex positions 10–42 (NAA type 6 = IEEE Registered Extended). PowerStore volumes store
// the WWN with the "naa." prefix, so the extracted value keeps it.
func (p *PowerstoreImporter) resolveRDM(deviceName string) (map[string]string, error) {
	wwn, err := extractWWN(deviceName)
	if err != nil {
		return nil, fmt.Errorf("PowerStore RDM resolution failed (DeviceName: %s): %w", deviceName, err)
	}
	name, err := p.volumeNameByField("wwn", wwn)
	if err != nil {
		return nil, fmt.Errorf("PowerStore RDM resolution failed (DeviceName: %s): %w", deviceName, err)
	}
	return map[string]string{annotationKey: name}, nil
}

// extractWWN returns the lowercase, "naa."-prefixed PowerStore WWN from either NAA or
// VML vSphere device name format.
func extractWWN(deviceName string) (string, error) {
	switch {
	case strings.HasPrefix(deviceName, "naa."):
		return strings.ToLower(deviceName), nil
	case strings.HasPrefix(deviceName, "vml."):
		hex := strings.TrimPrefix(deviceName, "vml.")
		if len(hex) < 42 {
			return "", fmt.Errorf("VML string too short to contain NAA: %s", deviceName)
		}
		naa := hex[10:42]
		if !strings.HasPrefix(naa, "6") {
			return "", fmt.Errorf("VML does not contain NAA type 6 identifier: %s", deviceName)
		}
		return "naa." + strings.ToLower(naa), nil
	default:
		return strings.ToLower(deviceName), nil
	}
}

// volumeNameByField queries GET /api/rest/volume?<field>=eq.<value>&select=name and
// returns the first match's name.
func (p *PowerstoreImporter) volumeNameByField(field, value string) (string, error) {
	queryURL := fmt.Sprintf("%s/api/rest/volume?select=name&%s=eq.%s", p.baseURL, field, url.QueryEscape(value))
	req, err := http.NewRequest(http.MethodGet, queryURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build PowerStore request: %w", err)
	}
	req.SetBasicAuth(p.user, p.pass)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("PowerStore volumes request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read PowerStore response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("PowerStore API returned %d: %s", resp.StatusCode, string(body))
	}

	var volumes []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &volumes); err != nil {
		return "", fmt.Errorf("failed to parse PowerStore response: %w", err)
	}
	if len(volumes) == 0 {
		return "", fmt.Errorf("no PowerStore volume found for %s=%s", field, value)
	}
	return volumes[0].Name, nil
}
