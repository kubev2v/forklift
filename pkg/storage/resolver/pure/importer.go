package pure

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/storage/resolver"
)

const (
	annotationKey   = "portworx.io/pure-volume-name"
	flashProviderID = "624a9370"
)

// PureImporter implements CsiImportPlugin for Pure FlashArray via REST API v2.
type PureImporter struct {
	hostname  string
	user      string
	pass      string
	client    *http.Client
	apiV1     string
	apiV2     string
	authToken string
}

func NewPureImporter(host, user, pass string, skipSSL bool) (*PureImporter, error) {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimRight(host, "/")
	if host == "" {
		return nil, fmt.Errorf("STORAGE_HOSTNAME must not be empty")
	}

	imp := &PureImporter{
		hostname: host,
		user:     user,
		pass:     pass,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: skipSSL}, //nolint:gosec
			},
		},
	}

	if err := imp.detectAPIVersions(); err != nil {
		return nil, fmt.Errorf("failed to detect Pure API versions at %s: %w", host, err)
	}
	if err := imp.authenticate(); err != nil {
		return nil, fmt.Errorf("failed to authenticate to Pure at %s: %w", host, err)
	}

	return imp, nil
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
		return nil, fmt.Errorf("Pure CSI import does not support VMDK disks")
	}
}

func (p *PureImporter) resolveVVol(vVolID string) (map[string]string, error) {
	uuid := strings.TrimPrefix(vVolID, "vvol:")

	baseURL := fmt.Sprintf("https://%s/api/%s/volumes/tags", p.hostname, p.apiV2)
	params := url.Values{}
	params.Set("resource_destroyed", "False")
	params.Set("namespaces", "vasa-integration.purestorage.com")
	params.Set("filter", fmt.Sprintf("key='PURE_VVOL_ID' AND value='%s'", uuid))

	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build tags request: %w", err)
	}
	req.Header.Set("x-auth-token", p.authToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Pure tags request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Pure tags response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Pure tags API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			Resource struct {
				Name string `json:"name"`
			} `json:"resource"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Pure tags response: %w", err)
	}
	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no Pure volume found for VVol ID %s", vVolID)
	}
	return map[string]string{annotationKey: result.Items[0].Resource.Name}, nil
}

func (p *PureImporter) resolveRDM(deviceName string) (map[string]string, error) {
	serial, err := extractSerialFromNAA(deviceName)
	if err != nil {
		return nil, fmt.Errorf("Pure RDM resolution failed (DeviceName: %s): %w", deviceName, err)
	}

	baseURL := fmt.Sprintf("https://%s/api/%s/volumes", p.hostname, p.apiV2)
	params := url.Values{}
	params.Set("filter", fmt.Sprintf("serial='%s'", serial))

	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build volumes request: %w", err)
	}
	req.Header.Set("x-auth-token", p.authToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Pure volumes request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Pure volumes response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Pure volumes API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Pure volumes response: %w", err)
	}
	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no Pure volume found for serial %s (DeviceName: %s)", serial, deviceName)
	}
	return map[string]string{annotationKey: result.Items[0].Name}, nil
}

// extractSerialFromNAA extracts the uppercase volume serial from a Pure NAA identifier.
// Pure NAA format: naa.624a9370<serial>
func extractSerialFromNAA(naa string) (string, error) {
	naa = strings.ToLower(strings.TrimSpace(naa))
	naa = strings.TrimPrefix(naa, "naa.")
	if !strings.HasPrefix(naa, strings.ToLower(flashProviderID)) {
		return "", fmt.Errorf("NAA %s does not have Pure OUI prefix %s", naa, flashProviderID)
	}
	serial := strings.TrimPrefix(naa, strings.ToLower(flashProviderID))
	if serial == "" {
		return "", fmt.Errorf("could not extract serial from NAA %s", naa)
	}
	return strings.ToUpper(serial), nil
}

// detectAPIVersions queries GET /api/api_version and selects the latest v1.x and v2.x.
func (p *PureImporter) detectAPIVersions() error {
	resp, err := p.client.Get(fmt.Sprintf("https://%s/api/api_version", p.hostname))
	if err != nil {
		return fmt.Errorf("failed to get API versions: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read API version response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API version request returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Version []string `json:"version"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse API version response: %w", err)
	}

	var v1s, v2s []string
	for _, v := range result.Version {
		if strings.HasPrefix(v, "1.") {
			v1s = append(v1s, v)
		} else if strings.HasPrefix(v, "2.") {
			v2s = append(v2s, v)
		}
	}
	if len(v1s) == 0 {
		return fmt.Errorf("no API v1.x versions found")
	}
	if len(v2s) == 0 {
		return fmt.Errorf("no API v2.x versions found")
	}

	sort.Slice(v1s, func(i, j int) bool { return compareVersions(v1s[i], v1s[j]) > 0 })
	sort.Slice(v2s, func(i, j int) bool { return compareVersions(v2s[i], v2s[j]) > 0 })

	p.apiV1 = v1s[0]
	p.apiV2 = v2s[0]
	return nil
}

// authenticate obtains an auth token via the two-step Pure flow:
// 1. POST /api/<v1>/auth/apitoken (username/password → api_token)
// 2. POST /api/<v2>/login (api_token → x-auth-token)
func (p *PureImporter) authenticate() error {
	apiToken, err := p.getAPIToken()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://%s/api/%s/login", p.hostname, p.apiV2), nil)
	if err != nil {
		return fmt.Errorf("failed to build login request: %w", err)
	}
	req.Header.Set("api-token", apiToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("Pure login failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Pure login returned %d: %s", resp.StatusCode, string(body))
	}

	token := resp.Header.Get("x-auth-token")
	if token == "" {
		return fmt.Errorf("no x-auth-token in Pure login response")
	}
	p.authToken = token
	return nil
}

func (p *PureImporter) getAPIToken() (string, error) {
	payload := fmt.Sprintf(`{"username":%q,"password":%q}`, p.user, p.pass)
	req, err := http.NewRequest("POST",
		fmt.Sprintf("https://%s/api/%s/auth/apitoken", p.hostname, p.apiV1),
		strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to build apitoken request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Pure apitoken request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read apitoken response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Pure apitoken returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		APIToken string `json:"api_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse apitoken response: %w", err)
	}
	if result.APIToken == "" {
		return "", fmt.Errorf("empty api_token in Pure response")
	}
	return result.APIToken, nil
}

// compareVersions compares two dot-separated version strings numerically.
// Returns >0 if a > b, <0 if a < b, 0 if equal.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		var aNum, bNum int
		fmt.Sscanf(aParts[i], "%d", &aNum)
		fmt.Sscanf(bParts[i], "%d", &bNum)
		if aNum != bNum {
			return aNum - bNum
		}
	}
	return len(aParts) - len(bParts)
}
