package hpe

import (
	"bytes"
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

// HpeImporter implements CsiImportPlugin for HPE Primera/3PAR/Alletra via WSAPI.
// Mirrors xcopy's Primera3ParCloner naming and credential-in-struct pattern.
type HpeImporter struct {
	baseURL string // e.g. "https://10.46.2.10:8080"
	user    string
	pass    string
	client  *http.Client
}

func NewHpeImporter(host, user, pass string, skipSSL bool) (*HpeImporter, error) {
	parsedURL, err := url.Parse(host)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("STORAGE_HOSTNAME must be a full URL with scheme (e.g. https://host:8080), got %q", host)
	}

	return &HpeImporter{
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

const AnnotationKey = "csi.hpe.com/importVolAsClone"

func (i *HpeImporter) Resolve(backing *resolver.DiskBacking) (map[string]string, bool, error) {
	if backing == nil {
		return nil, false, fmt.Errorf("nil disk backing")
	}
	switch resolver.DetectDiskType(backing) {
	case resolver.DiskTypeVVol:
		return i.resolveVVol(backing.VVolID)
	case resolver.DiskTypeRDM:
		return i.resolveRDM(backing.DeviceName)
	default:
		return nil, false, fmt.Errorf("HPE CSI import does not support VMDK disks")
	}
}

// resolveVVol maps a vSphere VVol BackingObjectId to an HPE volume name via WSAPI.
// BackingObjectId is normally "vvol:<uuid>", but HPE Primera reports it as "naa.<hex>" (a WWN).
// In the NAA case we strip the prefix and query by wwn, same as RDM resolution.
func (i *HpeImporter) resolveVVol(vVolId string) (map[string]string, bool, error) {
	var name string
	var found bool
	var err error
	if strings.HasPrefix(vVolId, "naa.") {
		wwn := strings.ToUpper(strings.TrimPrefix(vVolId, "naa."))
		name, found, err = i.volumeNameByFilter(fmt.Sprintf(`"wwn EQ %s"`, wwn))
	} else {
		uuid := strings.TrimPrefix(vVolId, "vvol:")
		name, found, err = i.volumeNameByFilter(fmt.Sprintf(`"uuid EQ %s"`, uuid))
	}
	if err != nil {
		return nil, false, fmt.Errorf("HPE VVol resolution failed (VVolID: %s): %w", vVolId, err)
	}
	if !found {
		return nil, false, nil
	}
	return map[string]string{AnnotationKey: name}, true, nil
}

// resolveRDM maps a vSphere RDM device name to an HPE volume name via WSAPI.
// DeviceName can be "naa.<hex>" or VML format "vml.<hex>". For VML, the HPE WWN
// is embedded at hex positions 10–42 (NAA type 6 = IEEE Registered Extended).
func (i *HpeImporter) resolveRDM(deviceName string) (map[string]string, bool, error) {
	wwn, err := extractWWN(deviceName)
	if err != nil {
		return nil, false, fmt.Errorf("HPE RDM resolution failed (DeviceName: %s): %w", deviceName, err)
	}
	name, found, err := i.volumeNameByFilter(fmt.Sprintf(`"wwn EQ %s"`, wwn))
	if err != nil {
		return nil, false, fmt.Errorf("HPE RDM resolution failed (DeviceName: %s): %w", deviceName, err)
	}
	if !found {
		return nil, false, nil
	}
	return map[string]string{AnnotationKey: name}, true, nil
}

// extractWWN returns the uppercase HPE WWN from either NAA or VML device name format.
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

func (i *HpeImporter) volumeNameByFilter(filter string) (string, bool, error) {
	sessionKey, err := i.getSessionKey()
	if err != nil {
		return "", false, err
	}
	defer i.deleteSessionKey(sessionKey)

	queryURL := fmt.Sprintf("%s/api/v1/volumes?query=%s", i.baseURL, url.PathEscape(filter))
	req, err := http.NewRequest("GET", queryURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("failed to build WSAPI request: %w", err)
	}
	req.Header.Set("X-HP3PAR-WSAPI-SessionKey", sessionKey)

	resp, err := i.client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("WSAPI volumes request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("failed to read WSAPI response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("WSAPI returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Total   int `json:"total"`
		Members []struct {
			Name string `json:"name"`
		} `json:"members"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", false, fmt.Errorf("failed to parse WSAPI response: %w", err)
	}
	if result.Total == 0 || len(result.Members) == 0 {
		return "", false, nil
	}
	return result.Members[0].Name, true, nil
}

// getSessionKey authenticates to POST /api/v1/credentials and returns the session key.
func (i *HpeImporter) getSessionKey() (string, error) {
	body, err := json.Marshal(map[string]string{"user": i.user, "password": i.pass})
	if err != nil {
		return "", fmt.Errorf("failed to encode credentials: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/credentials", i.baseURL), bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to build auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := i.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("WSAPI auth failed at %s: %w", i.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("WSAPI auth returned %d at %s", resp.StatusCode, i.baseURL)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse auth response: %w", err)
	}
	key, ok := result["key"]
	if !ok {
		return "", fmt.Errorf("no session key returned by HPE WSAPI at %s", i.baseURL)
	}
	return key, nil
}

func (i *HpeImporter) deleteSessionKey(sessionKey string) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/credentials/%s", i.baseURL, sessionKey), nil)
	if err != nil {
		return
	}
	resp, err := i.client.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}
