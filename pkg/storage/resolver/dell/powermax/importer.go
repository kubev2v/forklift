package powermax

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/storage/resolver"
	"github.com/vmware/govmomi/vim25/types"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// annotationKey is the PVC annotation the csi-powermax driver reads (import.storage.dell.com/volumeName).
// Its value is "<symmetrixID>/<deviceID>".
const annotationKey = "import.storage.dell.com/volumeName"

const (
	apiVersion = "100"
	// emcNAAPrefix is the NAA type-6 prefix of every Symmetrix/PowerMax device.
	emcNAAPrefix = "60000970"
)

// PowermaxImporter implements CsiImportPlugin and CsiImportPreparer for Dell PowerMax via Unisphere.
type PowermaxImporter struct {
	baseURL      string
	user         string
	pass         string
	symmetrixID  string
	storageClass string
	k8sClient    client.Client
	client       *http.Client
}

// NewPowermaxImporter validates the endpoint. Only https is accepted; skipSSL only skips certificate checks.
// The StorageClass is read in Prepare for importSourceStorageGroups, the value the csi-powermax driver checks.
func NewPowermaxImporter(host, user, pass, symmetrixID string, skipSSL bool, k8sClient client.Client, storageClass string) (*PowermaxImporter, error) {
	host = resolver.EnsureScheme(host)
	parsedURL, err := url.Parse(host)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return nil, fmt.Errorf("STORAGE_HOSTNAME must be an https URL (e.g. https://host:8443), got %q", host)
	}
	if symmetrixID == "" {
		return nil, fmt.Errorf("storage secret missing required key: POWERMAX_SYMMETRIX_ID")
	}
	return &PowermaxImporter{
		baseURL:      strings.TrimRight(host, "/"),
		user:         user,
		pass:         pass,
		symmetrixID:  symmetrixID,
		storageClass: storageClass,
		k8sClient:    k8sClient,
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

// Prepare removes the device from the Storage Groups behind its ESXi Masking View, including a Masking View on
// a parent of a cascaded group, and places it in the staging Storage Group. It refuses a device reached by more
// than one Masking View (shared RDM) before changing anything, and is idempotent.
func (p *PowermaxImporter) Prepare(backing *resolver.DiskBacking, annotations map[string]string) error {
	if backing == nil || resolver.DetectDiskType(backing) != resolver.DiskTypeRDM {
		return nil
	}
	staging, err := p.stagingStorageGroup()
	if err != nil {
		return err
	}
	symID, devID, ok := strings.Cut(annotations[annotationKey], "/")
	if !ok || symID != p.symmetrixID || devID == "" {
		return fmt.Errorf("unexpected import annotation %q", annotations[annotationKey])
	}
	// Prepare runs on every reconcile of the phase. Once the driver has adopted the device, the Masking View
	// belongs to the Kubernetes node, so it must not be touched.
	if identifier, _, err := p.volume(devID); err != nil || strings.HasPrefix(identifier, "csi-") {
		return err
	}
	mapped, views, err := p.mappedStorageGroups(devID)
	if err != nil {
		return err
	}
	if slices.Contains(mapped, staging) {
		return fmt.Errorf("staging Storage Group %s is part of a Masking View; use a Storage Group that is not masked", staging)
	}
	if len(views) > 1 {
		return fmt.Errorf("device %s is mapped through %d Masking Views %v, indicating a shared or clustered RDM; shared devices are not supported for import", devID, len(views), views)
	}
	for _, sg := range mapped {
		if err := p.editStorageGroup(sg, devID, false); err != nil {
			return fmt.Errorf("could not remove device %s from masking-view Storage Group %s: %w", devID, sg, err)
		}
	}
	_, sgs, err := p.volume(devID)
	if err != nil {
		return err
	}
	if !slices.Contains(sgs, staging) {
		if err := p.editStorageGroup(staging, devID, true); err != nil {
			for _, sg := range mapped {
				_ = p.editStorageGroup(sg, devID, true)
			}
			return fmt.Errorf("device %s could not be added to staging Storage Group %s; it may be unmapped and not in any approved group, manual recovery required: %w", devID, staging, err)
		}
	}
	mapped, _, err = p.mappedStorageGroups(devID)
	if err != nil {
		return err
	}
	if len(mapped) > 0 {
		return fmt.Errorf("device %s is still mapped through Storage Group %s", devID, mapped[0])
	}
	if _, sgs, err = p.volume(devID); err != nil {
		return err
	}
	if !slices.Contains(sgs, staging) {
		return fmt.Errorf("device %s is not in staging Storage Group %s after Prepare", devID, staging)
	}
	return nil
}

// stagingStorageGroup returns the single importSourceStorageGroups entry of the destination StorageClass.
// Forklift must not choose between several groups.
func (p *PowermaxImporter) stagingStorageGroup() (string, error) {
	if p.k8sClient == nil || p.storageClass == "" {
		return "", fmt.Errorf("PowerMax CSI import requires the destination StorageClass to read importSourceStorageGroups")
	}
	sc := &storagev1.StorageClass{}
	if err := p.k8sClient.Get(context.Background(), client.ObjectKey{Name: p.storageClass}, sc); err != nil {
		return "", fmt.Errorf("could not read StorageClass %s: %w", p.storageClass, err)
	}
	var groups []string
	for _, g := range strings.Split(sc.Parameters["importSourceStorageGroups"], ",") {
		if g = strings.TrimSpace(g); g != "" {
			groups = append(groups, g)
		}
	}
	if len(groups) != 1 {
		return "", fmt.Errorf("StorageClass %s must list exactly one importSourceStorageGroups entry for PowerMax import, found %d", p.storageClass, len(groups))
	}
	return groups[0], nil
}

// mappedStorageGroups returns the device's Storage Groups that are masked, directly or through a parent group,
// and the names of the Masking Views involved. The group-level count alone misses a Masking View on a parent.
func (p *PowermaxImporter) mappedStorageGroups(devID string) (mapped, views []string, err error) {
	_, sgs, err := p.volume(devID)
	if err != nil {
		return nil, nil, err
	}
	for _, name := range sgs {
		var sg struct {
			MaskingView []string `json:"maskingview"`
			Parents     []string `json:"parent_storage_group"`
		}
		if err := p.call(http.MethodGet, "/storagegroup/"+url.PathEscape(name), nil, &sg); err != nil {
			return nil, nil, err
		}
		found := sg.MaskingView
		for _, parent := range sg.Parents {
			var ps struct {
				MaskingView []string `json:"maskingview"`
			}
			if err := p.call(http.MethodGet, "/storagegroup/"+url.PathEscape(parent), nil, &ps); err != nil {
				return nil, nil, err
			}
			found = append(found, ps.MaskingView...)
		}
		if len(found) > 0 {
			mapped = append(mapped, name)
		}
		for _, v := range found {
			if !slices.Contains(views, v) {
				views = append(views, v)
			}
		}
	}
	return mapped, views, nil
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
	if _, err := hex.DecodeString(wwn[22:]); err != nil {
		return fmt.Errorf("malformed NAA identifier %q: device ID is not ASCII hex", wwn)
	}
	return nil
}

// extractWWN returns the uppercase PowerMax WWN from an NAA or VML device name.
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
	if err := p.call(http.MethodGet, "/volume?"+field+"="+url.QueryEscape(wwn), nil, &out); err != nil {
		return nil, err
	}
	var ids []string
	for _, r := range out.ResultList.Result {
		ids = append(ids, r.VolumeID)
	}
	return ids, nil
}

func (p *PowermaxImporter) volume(devID string) (identifier string, sgs []string, err error) {
	var out struct {
		Identifier         string   `json:"volume_identifier"`
		StorageGroupIDList []string `json:"storageGroupId"`
	}
	err = p.call(http.MethodGet, "/volume/"+url.PathEscape(devID), nil, &out)
	return out.Identifier, out.StorageGroupIDList, err
}

func (p *PowermaxImporter) editStorageGroup(sg, devID string, add bool) error {
	action := map[string]interface{}{"removeVolumeParam": map[string]interface{}{"volumeId": []string{devID}}}
	if add {
		action = map[string]interface{}{"expandStorageGroupParam": map[string]interface{}{
			"addVolumeParam": map[string]interface{}{"volumeId": []string{devID}, "create_new_volumes": false},
		}}
	}
	body := map[string]interface{}{"executionOption": "SYNCHRONOUS", "editStorageGroupActionParam": action}
	return p.call(http.MethodPut, "/storagegroup/"+url.PathEscape(sg), body, nil)
}

// call performs one Unisphere sloprovisioning request and decodes the JSON response into out.
func (p *PowermaxImporter) call(method, path string, body interface{}, out interface{}) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to encode Unisphere request: %w", err)
		}
		rd = bytes.NewReader(b)
	}
	reqURL := p.baseURL + "/univmax/restapi/" + apiVersion + "/sloprovisioning/symmetrix/" + url.PathEscape(p.symmetrixID) + path
	req, err := http.NewRequest(method, reqURL, rd)
	if err != nil {
		return fmt.Errorf("failed to build Unisphere request: %w", err)
	}
	req.SetBasicAuth(p.user, p.pass)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
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
