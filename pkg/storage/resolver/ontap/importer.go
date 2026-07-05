package ontap

import (
	"context"
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
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	annImportOriginalName = "trident.netapp.io/importOriginalName"
	annImportBackendUUID  = "trident.netapp.io/importBackendUUID"
	annNotManaged         = "trident.netapp.io/notManaged"

	ontapProviderID = "600a0980"

	DriverOntapSan        = "ontap-san"
	DriverOntapSanEconomy = "ontap-san-economy"
)

// detectDriverType resolves the Trident driver type for the given StorageClass and backend.
// It queries StorageClass parameters first, then scans existing PVs provisioned by the same
// Trident backend (matched by backendUUID), then falls back to secretDriverType if provided.
// Returns an error if none of the sources yield a result.
func detectDriverType(ctx context.Context, c client.Client, storageClassName, secretDriverType string) (string, error) {
	if c != nil {
		sc := &storagev1.StorageClass{}
		if err := c.Get(ctx, client.ObjectKey{Name: storageClassName}, sc); err == nil {
			if bt := sc.Parameters["backendType"]; bt == DriverOntapSan || bt == DriverOntapSanEconomy {
				return bt, nil
			}
		}
	}

	if secretDriverType != "" {
		klog.V(2).InfoS("ONTAP driver type not found in StorageClass backendType, falling back to TRIDENT_DRIVER from secret",
			"storageClass", storageClassName, "driver", secretDriverType)
		return secretDriverType, nil
	}

	return "", fmt.Errorf("ONTAP driver type could not be determined: set TRIDENT_DRIVER in the storage secret or configure backendType in StorageClass %q", storageClassName)
}

// OntapImporter implements CsiImportPlugin for NetApp ONTAP via direct REST API.
type OntapImporter struct {
	baseURL      string
	user         string
	pass         string
	svm          string
	backendUUID  string
	driverType   string // from TRIDENT_DRIVER secret key; used as fallback in detectDriverType
	storageClass string
	k8sClient    client.Client
	httpClient   *http.Client
}

func NewOntapImporter(host, user, pass, svm, backendUUID, driverType string, skipSSL bool, k8sClient client.Client, storageClass string) (*OntapImporter, error) {
	parsedURL, err := url.Parse(host)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("STORAGE_HOSTNAME must be a full URL with scheme (e.g. https://host), got %q", host)
	}
	if svm == "" {
		return nil, fmt.Errorf("ONTAP_SVM must not be empty")
	}
	if backendUUID == "" {
		return nil, fmt.Errorf("TRIDENT_BACKEND_UUID must not be empty")
	}

	return &OntapImporter{
		baseURL:      strings.TrimRight(host, "/"),
		user:         user,
		pass:         pass,
		svm:          svm,
		backendUUID:  backendUUID,
		driverType:   driverType,
		storageClass: storageClass,
		k8sClient:    k8sClient,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: skipSSL}, //nolint:gosec
			},
		},
	}, nil
}

// Resolve returns the PVC annotations Trident needs to import the source array volume.
func (o *OntapImporter) Resolve(backing *resolver.DiskBacking) (map[string]string, error) {
	if backing == nil {
		return nil, fmt.Errorf("nil disk backing")
	}
	switch resolver.DetectDiskType(backing) {
	case resolver.DiskTypeVVol:
		return o.resolveVVol(backing.VVolID)
	case resolver.DiskTypeRDM:
		return o.resolveRDM(backing.DeviceName)
	default:
		return nil, fmt.Errorf("ONTAP CSI import does not support VMDK disks")
	}
}

func (o *OntapImporter) resolveVVol(vvolID string) (map[string]string, error) {
	klog.V(2).InfoS("ONTAP VVol CSI import not yet supported, deferring to xcopy", "vvolID", vvolID)
	return nil, nil //nolint:nilnil
}

func (o *OntapImporter) resolveRDM(deviceName string) (map[string]string, error) {
	serial, err := extractSerialFromNAA(deviceName)
	if err != nil {
		return nil, fmt.Errorf("ONTAP RDM resolution failed (DeviceName: %s): %w", deviceName, err)
	}

	lunPath, err := o.queryLUNBySerial(serial)
	if err != nil {
		return nil, fmt.Errorf("ONTAP RDM resolution failed (serial: %s): %w", serial, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	driverType, err := detectDriverType(ctx, o.k8sClient, o.storageClass, o.driverType)
	if err != nil {
		return nil, fmt.Errorf("ONTAP RDM resolution failed: %w", err)
	}

	importName, err := formatImportName(lunPath, driverType)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		annImportOriginalName: importName,
		annImportBackendUUID:  o.backendUUID,
		annNotManaged:         "true",
	}, nil
}

func (o *OntapImporter) queryLUNBySerial(serial string) (string, error) {
	queryURL := fmt.Sprintf("%s/api/storage/luns?serial_number=%s&svm.name=%s&fields=name",
		o.baseURL, url.QueryEscape(serial), url.QueryEscape(o.svm))

	req, err := http.NewRequest(http.MethodGet, queryURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build ONTAP request: %w", err)
	}
	req.SetBasicAuth(o.user, o.pass)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ONTAP REST request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read ONTAP response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ONTAP REST returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		NumRecords int `json:"num_records"`
		Records    []struct {
			Name string `json:"name"`
		} `json:"records"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse ONTAP response: %w", err)
	}
	if result.NumRecords == 0 || len(result.Records) == 0 {
		return "", fmt.Errorf("no ONTAP LUN found for serial %s on SVM %s", serial, o.svm)
	}
	if result.NumRecords > 1 {
		return "", fmt.Errorf("multiple ONTAP LUNs (%d) found for serial %s on SVM %s", result.NumRecords, serial, o.svm)
	}
	return result.Records[0].Name, nil
}

// formatImportName builds the Trident importOriginalName from an ONTAP LUN path.
// LUN path format: /vol/<flexvol>/<lun>
// ontap-san:         returns "<flexvol>" (one LUN per FlexVol)
// ontap-san-economy: returns "<flexvol>/<lun>" (multiple LUNs per FlexVol)
func formatImportName(lunPath, driverType string) (string, error) {
	// ONTAP LUN path: /vol/<flexvol>/<lun> → split produces ["", "vol", "<flexvol>", "<lun>"]
	parts := strings.Split(lunPath, "/")
	if len(parts) < 4 || parts[1] != "vol" || parts[2] == "" || parts[3] == "" {
		return "", fmt.Errorf("invalid ONTAP LUN path format (expected /vol/<flexvol>/<lun>): %s", lunPath)
	}
	if driverType == DriverOntapSanEconomy {
		return parts[2] + "/" + parts[3], nil
	}
	return parts[2], nil
}

// extractSerialFromNAA extracts the ASCII serial number from an ONTAP device name.
// Supports both NAA format (naa.600a0980<hex>) and VML format (vml.<descriptor>).
// The xcopy code encodes serials as fmt.Sprintf("naa.%s%x", "600a0980", serialString),
// so we reverse: strip prefix, hex-decode the remainder to get the ASCII serial.
func extractSerialFromNAA(deviceName string) (string, error) {
	deviceName = strings.ToLower(strings.TrimSpace(deviceName))

	var naaHex string
	switch {
	case strings.HasPrefix(deviceName, "vml."):
		vmlHex := strings.TrimPrefix(deviceName, "vml.")
		if len(vmlHex) < 42 {
			return "", fmt.Errorf("VML string too short to contain NAA: %s", deviceName)
		}
		naaHex = vmlHex[10:42]
	case strings.HasPrefix(deviceName, "naa."):
		naaHex = strings.TrimPrefix(deviceName, "naa.")
	default:
		naaHex = deviceName
	}

	if !strings.HasPrefix(naaHex, ontapProviderID) {
		return "", fmt.Errorf("device %s does not have ONTAP OUI prefix %s", deviceName, ontapProviderID)
	}
	hexSerial := strings.TrimPrefix(naaHex, ontapProviderID)
	if hexSerial == "" {
		return "", fmt.Errorf("could not extract serial from device %s", deviceName)
	}
	decoded, err := hex.DecodeString(hexSerial)
	if err != nil {
		return "", fmt.Errorf("failed to hex-decode serial from device %s: %w", deviceName, err)
	}
	return string(decoded), nil
}
