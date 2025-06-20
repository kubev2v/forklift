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

const FlashSystemProviderIDPrefix = "naa.6005076"

const APIVersion = "1.0"

type AuthResponse struct {
	Token string `json:"token"`
}

type FlashSystemHost struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	PortCount int    `json:"port_count"`
	Type      string `json:"type"`
	WWPN      string `json:"wwpn"`
}

type FlashSystemVolume struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	UID    string `json:"uid"` // Unique Identification Number, used for NAA.
	Status string `json:"status"`
	Size   string `json:"size"`
}

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

type FlashSystemAPIClient struct {
	ManagementIP string
	httpClient   *http.Client
	authToken    string // Session token from /auth
}

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
func NewFlashSystemClonner(managementIP, username, password string, sslSkipVerify bool) (*FlashSystemClonner, error) {
	client, err := NewFlashSystemAPIClient(managementIP, username, password, sslSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create FlashSystem API client: %w", err)
	}
	return &FlashSystemClonner{api: client}, nil
}

// EnsureClonnerIgroup creates or finds a host with the given IQNs.
func (c *FlashSystemClonner) EnsureClonnerIgroup(initiatorGroup string, clonnerIqns []string) (populator.MappingContext, error) {
	klog.Infof("Ensuring initiator group (Host) '%s' exists with IQNs: %v", initiatorGroup, clonnerIqns)
	ctx := make(populator.MappingContext)

	path := fmt.Sprintf("/host/%s", initiatorGroup)
	bodyBytes, statusCode, err := c.api.makeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var host FlashSystemHost
	if statusCode == http.StatusOK {
		var hosts []FlashSystemHost // API returns list even for specific query
		if err := json.Unmarshal(bodyBytes, &hosts); err != nil || len(hosts) == 0 {
			return nil, fmt.Errorf("failed to unmarshal existing host '%s': %w. Body: %s", initiatorGroup, err, string(bodyBytes))
		}
		host = hosts[0]
		klog.Infof("Host '%s' already exists with ID '%s'.", host.Name, host.ID)
	} else if statusCode == http.StatusNotFound || statusCode == http.StatusBadRequest { // 400 can mean not found too
		klog.Infof("Host '%s' does not exist, creating it.", initiatorGroup)

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
	vdiskID := targetLUN.Name

	klog.Infof("Mapping LUN (VDisk ID '%s') to Host '%s' (Host ID '%s')", vdiskID, initiatorGroup, hostID)

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

	payload := map[string]string{
		"host_id":  hostID,
		"vdisk_id": vdiskID,
	}
	mapBody, mapStatus, mapErr := c.api.makeRequest("POST", "/hostvdiskmap", payload)
	if mapErr != nil {
		return populator.LUN{}, mapErr
	}
	if mapStatus != http.StatusOK && mapStatus != http.StatusCreated {
		return populator.LUN{}, fmt.Errorf("failed to map VDisk '%s' to Host '%s', status: %d, body: %s", vdiskID, hostID, mapStatus, string(mapBody))
	}
	klog.Infof("Successfully initiated mapping of VDisk '%s' to Host '%s'.", vdiskID, hostID)
	return targetLUN, nil
}

func (c *FlashSystemClonner) UnMap(initiatorGroup string, targetLUN populator.LUN, context populator.MappingContext) error {
	hostID, ok := context["host_id"].(string)
	if !ok || hostID == "" {
		return fmt.Errorf("host_id not found or invalid in mapping context for initiator group '%s'", initiatorGroup)
	}
	vdiskID := targetLUN.Name

	klog.Infof("Unmapping LUN (VDisk ID '%s') from Host '%s' (Host ID '%s')", vdiskID, initiatorGroup, hostID)

	payload := map[string]string{
		"host_id":  hostID,
		"vdisk_id": vdiskID,
	}

	unmapBody, unmapStatus, unmapErr := c.api.makeRequest("DELETE", "/hostvdiskmap", payload)
	if unmapErr != nil {
		return unmapErr
	}

	if unmapStatus != http.StatusOK && unmapStatus != http.StatusNoContent {
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
func (c *FlashSystemClonner) ResolveVolumeHandleToLUN(volumeHandle string) (populator.LUN, error) {
	klog.Infof("Resolving volume handle '%s' to LUN details", volumeHandle)

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

	var vdisks []FlashSystemVolume
	if err := json.Unmarshal(bodyBytes, &vdisks); err != nil || len(vdisks) == 0 {
		return populator.LUN{}, fmt.Errorf("failed to unmarshal volume response for '%s': %w. Body: %s", volumeHandle, err, string(bodyBytes))
	}

	vdiskDetails := vdisks[0]

	vdiskUID := strings.ToLower(vdiskDetails.UID)
	if vdiskUID == "" {
		return populator.LUN{}, fmt.Errorf("resolved volume '%s' has an empty UID", vdiskDetails.Name)
	}
	naaDeviceID := FlashSystemProviderIDPrefix + vdiskUID

	lun := populator.LUN{
		Name:         vdiskDetails.ID,
		VolumeHandle: volumeHandle,
		SerialNumber: vdiskDetails.UID,
		NAA:          naaDeviceID,
	}

	klog.Infof("Resolved volume handle '%s' to LUN: Name(ID)=%s, SN(UID)=%s, NAA=%s",
		volumeHandle, lun.Name, lun.SerialNumber, lun.NAA)
	return lun, nil
}
