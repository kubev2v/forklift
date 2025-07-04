package flashsystem

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"k8s.io/klog/v2"
)

// FlashSystemProviderIDPrefix is the standard NAA prefix for IBM LUNs.
const FlashSystemProviderIDPrefix = "naa.6005076"

// APIVersion is the API version specified in the provided documentation (RESTfulAPIv2.pdf).
const APIVersion = "1.0"

// AuthResponse models the JSON response from the /auth endpoint.
type AuthResponse struct {
	Token string `json:"token"`
}

// FlashSystemHost models the JSON object for a host (see Table 22, page 28).
type FlashSystemHost struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	PortCount int    `json:"port_count"`
	Type      string `json:"type"`
	WWPN      string `json:"wwpn"`
	// Other fields like WWPN, node_logged_in_count, etc., are available but not used in this implementation.
}

// FlashSystemVolume models the JSON object for a VDisk (see Table 16, page 18).
type FlashSystemVolume struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	UID    string `json:"uid"` // Unique Identification Number, used for NAA.
	Status string `json:"status"`
	Size   string `json:"size"`
}

// FlashSystemVolumeHostMapping models the JSON object for a host-vdisk mapping (see Table 24, page 29).
type FlashSystemVolumeHostMapping struct {
	HostID      string `json:"id"` // This is the Host ID
	HostName    string `json:"name"`
	VDiskID     string `json:"vdisk_id"`
	VDiskName   string `json:"vdisk_name"`
	VDiskUID    string `json:"vdisk_UID"`
	SCSIID      string `json:"scsi_id"`
	IOGroupID   string `json:"io_group_id"`
	IOGroupName string `json:"io_group_name"`
}

// FlashSystemAPIClient handles communication with the FlashSystem REST API.
type FlashSystemAPIClient struct {
	ManagementIP string
	httpClient   *http.Client
	authToken    string // Session token from /auth
}

// NewFlashSystemAPIClient creates and authenticates a new API client.
// Authentication follows the procedure on pages 8 and 53 of the documentation.
func NewFlashSystemAPIClient(managementIP, username, password string, sslSkipVerify bool) (*FlashSystemAPIClient, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: sslSkipVerify},
	}
	httpClient := &http.Client{Transport: transport, Timeout: time.Second * 60}

	authURL := fmt.Sprintf("https://%s:9443/%s/auth", managementIP, APIVersion)
	req, err := http.NewRequest("GET", authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth request: %w", err)
	}
	req.Header.Set("X-Auth-Username", username)
	req.Header.Set("X-Auth-Password", password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	klog.Infof("Attempting to authenticate with FlashSystem at %s for user %s", managementIP, username)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send auth request to FlashSystem: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read auth response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FlashSystem authentication failed. Status: %s, Body: %s", resp.Status, string(bodyBytes))
	}

	var authResp AuthResponse
	if err := json.Unmarshal(bodyBytes, &authResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth token response: %w. Body: %s", err, string(bodyBytes))
	}
	if authResp.Token == "" {
		return nil, fmt.Errorf("FlashSystem authentication successful but no token found in response")
	}

	klog.Infof("Successfully authenticated with FlashSystem and obtained session token.")
	client := &FlashSystemAPIClient{
		ManagementIP: managementIP,
		httpClient:   httpClient,
		authToken:    authResp.Token,
	}
	return client, nil
}

// makeRequest is a helper to make authenticated HTTP requests.
func (c *FlashSystemAPIClient) makeRequest(method, path string, payload interface{}) ([]byte, int, error) {
	fullURL := fmt.Sprintf("https://%s:9443/%s%s", c.ManagementIP, APIVersion, path)
	klog.V(4).Infof("FlashSystem API Request: %s %s", method, fullURL)

	var reqBody *bytes.Buffer
	if payload != nil {
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal payload for %s: %w", fullURL, err)
		}
		reqBody = bytes.NewBuffer(jsonPayload)
		klog.V(5).Infof("Request Payload: %s", string(jsonPayload))
	} else {
		reqBody = bytes.NewBuffer([]byte{})
	}

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request for %s: %w", fullURL, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Auth-Token", c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request to %s failed: %w", fullURL, err)
	}
	defer resp.Body.Close()

	respBodyBytes, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body from %s: %w", fullURL, readErr)
	}
	klog.V(5).Infof("Response Status: %s, Body: %s", resp.Status, string(respBodyBytes))

	return respBodyBytes, resp.StatusCode, nil
}

// FlashSystemClonner implements the populator.StorageApi interface.
type FlashSystemClonner struct {
	api *FlashSystemAPIClient
}

// NewFlashSystemClonner creates a new FlashSystemClonner.
func NewFlashSystemClonner(managementIP, username, password string, sslSkipVerify bool) (FlashSystemClonner, error) {
	client, err := NewFlashSystemAPIClient(managementIP, username, password, sslSkipVerify)
	if err != nil {
		return FlashSystemClonner{}, fmt.Errorf("failed to create FlashSystem API client: %w", err)
	}
	return FlashSystemClonner{api: client}, nil
}

// EnsureClonnerIgroup creates or finds a host with the given IQNs.
// This implements creating a Host object as described on page 28 of the documentation.
func (c *FlashSystemClonner) EnsureClonnerIgroup(initiatorGroup string, clonnerIqns []string) (populator.MappingContext, error) {
	klog.Infof("Ensuring initiator group (Host) '%s' exists with IQNs: %v", initiatorGroup, clonnerIqns)
	ctx := make(populator.MappingContext)

	// Step 1: Check if Host exists using `GET /host/<name>`
	path := fmt.Sprintf("/host/%s", initiatorGroup)
	bodyBytes, statusCode, err := c.api.makeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var host FlashSystemHost
	// If host is found, unmarshal it. If not found (404), create it.
	if statusCode == http.StatusOK {
		var hosts []FlashSystemHost // API returns list even for specific query
		if err := json.Unmarshal(bodyBytes, &hosts); err != nil || len(hosts) == 0 {
			return nil, fmt.Errorf("failed to unmarshal existing host '%s': %w. Body: %s", initiatorGroup, err, string(bodyBytes))
		}
		host = hosts[0]
		klog.Infof("Host '%s' already exists with ID '%s'.", host.Name, host.ID)
		// NOTE: The provided API docs do not specify a method to ADD an IQN to an existing host.
		// We assume that if the host exists, it is correctly configured.
	} else if statusCode == http.StatusNotFound || statusCode == http.StatusBadRequest { // 400 can mean not found too
		klog.Infof("Host '%s' does not exist, creating it.", initiatorGroup)

		// Step 2: Create the host using `POST /host`
		// Per Table 23, page 28, 'login_string' provides the initiator name for iSCSI.
		// The example for WWPNs is colon-separated, we apply the same logic for IQNs.
		loginString := strings.Join(clonnerIqns, ":")
		payload := map[string]string{
			"name":         initiatorGroup,
			"login_string": loginString,
		}

		createBody, createStatusCode, createErr := c.api.makeRequest("POST", "/host", payload)
		if createErr != nil {
			return nil, createErr
		}
		if createStatusCode != http.StatusOK && createStatusCode != http.StatusCreated {
			return nil, fmt.Errorf("failed to create host '%s', status code: %d, body: %s", initiatorGroup, createStatusCode, string(createBody))
		}
		// After creation, we need to fetch the host again to get its details, especially the ID.
		refetchBody, refetchStatusCode, refetchErr := c.api.makeRequest("GET", fmt.Sprintf("/host/%s", initiatorGroup), nil)
		if refetchErr != nil || refetchStatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to re-fetch host '%s' after creation: %w", initiatorGroup, refetchErr)
		}
		var hosts []FlashSystemHost
		if err := json.Unmarshal(refetchBody, &hosts); err != nil || len(hosts) == 0 {
			return nil, fmt.Errorf("failed to unmarshal newly created host '%s': %w", initiatorGroup, err)
		}
		host = hosts[0]
		klog.Infof("Host '%s' created successfully with ID '%s'.", host.Name, host.ID)
	} else {
		return nil, fmt.Errorf("unexpected status code %d when checking for host '%s': %s", statusCode, initiatorGroup, string(bodyBytes))
	}

	ctx["host_id"] = host.ID
	ctx["host_name"] = host.Name
	klog.Infof("Successfully ensured initiator group (Host) '%s'", initiatorGroup)
	return ctx, nil
}

// Map maps a VDisk to a Host. Implements `POST /hostvdiskmap` from page 29.
func (c *FlashSystemClonner) Map(initiatorGroup string, targetLUN populator.LUN, context populator.MappingContext) (populator.LUN, error) {
	hostID, ok := context["host_id"].(string)
	if !ok || hostID == "" {
		return populator.LUN{}, fmt.Errorf("host_id not found or invalid in mapping context for initiator group '%s'", initiatorGroup)
	}
	// For mapping, we need the VDisk ID, which is part of the LUN object from ResolveVolumeHandleToLUN.
	vdiskID := targetLUN.Name // The LUN.Name field should hold the VDisk ID for this provider.

	klog.Infof("Mapping LUN (VDisk ID '%s') to Host '%s' (Host ID '%s')", vdiskID, initiatorGroup, hostID)

	// Step 1: Check if mapping already exists.
	allMappings, status, err := c.api.makeRequest("GET", "/hostvdiskmap", nil)
	if err != nil || status != http.StatusOK {
		return populator.LUN{}, fmt.Errorf("failed to get all hostvdiskmaps: %w, status: %d", err, status)
	}
	var mappings []FlashSystemVolumeHostMapping
	if err := json.Unmarshal(allMappings, &mappings); err == nil {
		for _, m := range mappings {
			if m.HostID == hostID && m.VDiskID == vdiskID {
				klog.Infof("LUN (VDisk ID '%s') is already mapped to Host ID '%s'. SCSI ID: %s", vdiskID, hostID, m.SCSIID)
				// TODO: this is wrong????
				targetLUN.IQN = m.SCSIID
				return targetLUN, nil
			}
		}
	}

	// Step 2: Create the mapping using `POST /hostvdiskmap` (Table 25, page 29).
	payload := map[string]string{
		"host_id":  hostID,
		"vdisk_id": vdiskID,
	}
	// If a specific SCSI ID is desired and available in targetLUN, it can be added.
	// if targetLUN.SCSIID != "" { payload["scsi"] = targetLUN.SCSIID }

	mapBody, mapStatus, mapErr := c.api.makeRequest("POST", "/hostvdiskmap", payload)
	if mapErr != nil {
		return populator.LUN{}, mapErr
	}
	if mapStatus != http.StatusOK && mapStatus != http.StatusCreated {
		return populator.LUN{}, fmt.Errorf("failed to map VDisk '%s' to Host '%s', status: %d, body: %s", vdiskID, hostID, mapStatus, string(mapBody))
	}
	klog.Infof("Successfully initiated mapping of VDisk '%s' to Host '%s'.", vdiskID, hostID)
	// The API does not return the mapping object on creation, so we can't get the SCSI ID without another GET.
	// For now, we assume success without updating the SCSIID.
	return targetLUN, nil
}

// UnMap unmaps a VDisk from a Host. Implements `DELETE /hostvdiskmap` with a body, as per Table 26, page 29.
func (c *FlashSystemClonner) UnMap(initiatorGroup string, targetLUN populator.LUN, context populator.MappingContext) error {
	hostID, ok := context["host_id"].(string)
	if !ok || hostID == "" {
		return fmt.Errorf("host_id not found or invalid in mapping context for initiator group '%s'", initiatorGroup)
	}
	vdiskID := targetLUN.Name // VDisk ID from the LUN object.

	klog.Infof("Unmapping LUN (VDisk ID '%s') from Host '%s' (Host ID '%s')", vdiskID, initiatorGroup, hostID)

	// Per Table 26, page 29, the DELETE operation requires a body with host_id and vdisk_id.
	payload := map[string]string{
		"host_id":  hostID,
		"vdisk_id": vdiskID,
	}

	unmapBody, unmapStatus, unmapErr := c.api.makeRequest("DELETE", "/hostvdiskmap", payload)
	if unmapErr != nil {
		return unmapErr
	}

	// 200 OK or 204 No Content are typical for success.
	if unmapStatus != http.StatusOK && unmapStatus != http.StatusNoContent {
		// It's possible the API returns 404 if mapping doesn't exist, which is idempotent.
		if unmapStatus == http.StatusNotFound {
			klog.Infof("Mapping for VDisk '%s' to Host '%s' did not exist.", vdiskID, hostID)
			return nil
		}
		return fmt.Errorf("failed to unmap VDisk '%s' from Host '%s', status: %d, body: %s", vdiskID, hostID, unmapStatus, string(unmapBody))
	}

	klog.Infof("Successfully unmapped LUN (VDisk ID '%s') from Host '%s'", vdiskID, hostID)
	return nil
}

// CurrentMappedGroups returns the initiator groups (Host names) a VDisk is mapped to.
func (c *FlashSystemClonner) CurrentMappedGroups(targetLUN populator.LUN, context populator.MappingContext) ([]string, error) {
	vdiskUID := targetLUN.SerialNumber // VDisk UID is the unique serial number.
	klog.Infof("Getting current mapped groups for LUN (VDisk UID '%s')", vdiskUID)

	// The API GET /hostvdiskmap returns all mappings. We must filter on the client side.
	bodyBytes, status, err := c.api.makeRequest("GET", "/hostvdiskmap", nil)
	if err != nil || status != http.StatusOK {
		return nil, fmt.Errorf("failed to get hostvdiskmaps: %w, status: %d, body: %s", err, status, string(bodyBytes))
	}

	var mappings []FlashSystemVolumeHostMapping
	if err := json.Unmarshal(bodyBytes, &mappings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal mappings: %w. Body: %s", err, string(bodyBytes))
	}

	var mappedGroups []string
	for _, m := range mappings {
		if m.VDiskUID == vdiskUID {
			mappedGroups = append(mappedGroups, m.HostName)
		}
	}
	// Remove duplicates
	groupSet := make(map[string]bool)
	uniqueGroups := []string{}
	for _, group := range mappedGroups {
		if !groupSet[group] {
			groupSet[group] = true
			uniqueGroups = append(uniqueGroups, group)
		}
	}

	klog.Infof("LUN (VDisk UID '%s') is mapped to groups: %v", vdiskUID, uniqueGroups)
	return uniqueGroups, nil
}

// ResolveVolumeHandleToLUN resolves a volumeHandle (VDisk name or ID) to a LUN object.
// Implements `GET /vdisk/<id_or_name>` from page 18.
func (c *FlashSystemClonner) ResolveVolumeHandleToLUN(volumeHandle string) (populator.LUN, error) {
	klog.Infof("Resolving volume handle '%s' to LUN details", volumeHandle)

	// The API allows getting a vdisk by its ID or name.
	path := fmt.Sprintf("/vdisk/%s", volumeHandle)
	bodyBytes, statusCode, err := c.api.makeRequest("GET", path, nil)
	if err != nil {
		return populator.LUN{}, err
	}

	if statusCode == http.StatusNotFound || statusCode == http.StatusBadRequest { // API might return 400 for not found
		return populator.LUN{}, fmt.Errorf("volume with handle '%s' not found", volumeHandle)
	}
	if statusCode != http.StatusOK {
		return populator.LUN{}, fmt.Errorf("failed to get volume '%s', status: %d, body: %s", volumeHandle, statusCode, string(bodyBytes))
	}

	var vdisks []FlashSystemVolume // API returns a list even for a specific query
	if err := json.Unmarshal(bodyBytes, &vdisks); err != nil || len(vdisks) == 0 {
		return populator.LUN{}, fmt.Errorf("failed to unmarshal volume response for '%s': %w. Body: %s", volumeHandle, err, string(bodyBytes))
	}

	vdiskDetails := vdisks[0]

	// Construct NAA identifier from the VDisk UID (see Table 16, page 18).
	vdiskUID := strings.ToLower(vdiskDetails.UID)
	if vdiskUID == "" {
		return populator.LUN{}, fmt.Errorf("resolved volume '%s' has an empty UID", vdiskDetails.Name)
	}
	naaDeviceID := FlashSystemProviderIDPrefix + vdiskUID

	// For this provider, we store the VDisk ID in the LUN.Name field,
	// as it's needed for Map/Unmap operations.
	lun := populator.LUN{
		Name:         vdiskDetails.ID,  // VDisk ID, required for mapping.
		VolumeHandle: volumeHandle,     // The original input handle.
		SerialNumber: vdiskDetails.UID, // VDisk UID, which is the unique serial.
		NAA:          naaDeviceID,
	}

	klog.Infof("Resolved volume handle '%s' to LUN: Name(ID)=%s, SN(UID)=%s, NAA=%s",
		volumeHandle, lun.Name, lun.SerialNumber, lun.NAA)
	return lun, nil
}
