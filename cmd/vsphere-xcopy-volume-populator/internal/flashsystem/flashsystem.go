package flashsystem

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"k8s.io/klog/v2"
)

// FlashSystemProviderIDPrefix is the standard NAA prefix for IBM LUNs.
const FlashSystemProviderIDPrefix = "naa.6005076"

// isHexWWPN checks if a string looks like a hexadecimal WWPN
func isHexWWPN(s string) bool {
	// Remove common separators and prefixes
	cleaned := strings.ReplaceAll(strings.ReplaceAll(s, ":", ""), "-", "")
	cleaned = strings.TrimPrefix(cleaned, "fc.")

	// Check if it's 16 hex characters (typical WWPN length)
	hexPattern := regexp.MustCompile(`^[0-9a-fA-F]{16}$`)
	return hexPattern.MatchString(cleaned)
}

// extractWWPNsFromFCFormat extracts individual WWPNs from fc.WWPN1:WWPN2 format
// Uses the second part (after colon) as the real WWPN
func extractWWPNsFromFCFormat(fcStrings []string) []string {
	var wwpns []string
	for _, fcStr := range fcStrings {
		if strings.HasPrefix(fcStr, "fc.") {
			// Remove "fc." prefix and split by ":"
			parts := strings.Split(strings.TrimPrefix(fcStr, "fc."), ":")
			// Use the SECOND part (after colon) - this is the real WWPN
			if len(parts) > 1 && len(parts[1]) == 16 && regexp.MustCompile(`^[0-9a-fA-F]+$`).MatchString(parts[1]) {
				wwpns = append(wwpns, parts[1])
				klog.Infof("Extracted WWPN (second half): %s from %s", parts[1], fcStr)
			}
		}
	}
	return wwpns
}

// AuthResponse models the JSON response from the /auth endpoint.
type AuthResponse struct {
	Token string `json:"token"`
}

type FlashSystemHost struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type FlashSystemVolume struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	VdiskUID string `json:"vdisk_UID"` // Unique Identification Number, used for NAA.
}

type FlashSystemVolumeHostMapping struct {
	HostID   string `json:"id"` // This is the Host ID
	HostName string `json:"name"`
}

type HostPort struct {
	HostID   string `json:"host_id"`
	HostName string `json:"host_name"`
	WWPN     string `json:"WWPN"` // API returns uppercase WWPN
	IQN      string `json:"iscsi_name"`
}

// FlashSystemAPIClient handles communication with the FlashSystem REST API.
type FlashSystemAPIClient struct {
	ManagementIP string
	httpClient   *http.Client
	authToken    string       // Session token from /auth
	username     string       // Store for re-authentication
	password     string       // Store for re-authentication
	authMutex    sync.RWMutex // Protect token operations
}

// NewFlashSystemAPIClient creates and authenticates a new API client.
func NewFlashSystemAPIClient(managementIP, username, password string, sslSkipVerify bool) (*FlashSystemAPIClient, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: sslSkipVerify},
	}
	httpClient := &http.Client{Transport: transport, Timeout: time.Second * 60}

	client := &FlashSystemAPIClient{
		ManagementIP: managementIP,
		httpClient:   httpClient,
		username:     username,
		password:     password,
	}

	// Initial authentication
	if err := client.authenticate(); err != nil {
		return nil, fmt.Errorf("initial authentication failed: %w", err)
	}

	return client, nil
}

// authenticate handles the authentication process using v1 API best practices
func (c *FlashSystemAPIClient) authenticate() error {
	authURL := fmt.Sprintf("https://%s:7443/rest/v1/auth", c.ManagementIP)

	// FlashSystem expects username and password via HTTP headers, not JSON body
	req, err := http.NewRequest("POST", authURL, bytes.NewBuffer([]byte{}))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Auth-Username", c.username)
	req.Header.Set("X-Auth-Password", c.password)

	klog.Infof("Attempting to authenticate with FlashSystem at %s for user %s", c.ManagementIP, c.username)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send auth request to FlashSystem: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read auth response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("FlashSystem authentication failed. Status: %s, Body: %s", resp.Status, string(bodyBytes))
	}

	var authResp AuthResponse
	if err := json.Unmarshal(bodyBytes, &authResp); err != nil {
		return fmt.Errorf("failed to unmarshal auth token response: %w. Body: %s", err, string(bodyBytes))
	}

	if authResp.Token == "" {
		return fmt.Errorf("FlashSystem authentication successful but no token found in response")
	}

	c.authMutex.Lock()
	c.authToken = authResp.Token
	c.authMutex.Unlock()

	klog.Infof("Successfully authenticated with FlashSystem and obtained session token.")
	return nil
}

// makeRequest is a helper to make authenticated HTTP requests with automatic token refresh.
func (c *FlashSystemAPIClient) makeRequest(method, path string, payload interface{}) ([]byte, int, error) {
	// Try the request first, and handle 403 (token expiry) by re-authenticating
	respBodyBytes, statusCode, err := c.doRequest(method, path, payload)

	// Handle 403 Forbidden - token expired, re-authenticate and retry once
	if statusCode == http.StatusForbidden && err == nil {
		klog.Infof("Received 403 Forbidden, token likely expired. Re-authenticating...")
		if authErr := c.authenticate(); authErr != nil {
			return nil, statusCode, fmt.Errorf("re-authentication failed: %w", authErr)
		}
		// Retry the request with new token
		return c.doRequest(method, path, payload)
	}

	return respBodyBytes, statusCode, err
}

// doRequest performs the actual HTTP request
func (c *FlashSystemAPIClient) doRequest(method, path string, payload interface{}) ([]byte, int, error) {
	fullURL := fmt.Sprintf("https://%s:7443/rest/v1%s", c.ManagementIP, path)
	klog.Infof("FlashSystem API Request: %s %s", method, fullURL)

	var reqBody *bytes.Buffer
	if payload != nil {
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal payload for %s: %w", fullURL, err)
		}
		reqBody = bytes.NewBuffer(jsonPayload)
		klog.Infof("Request Payload JSON: %s", string(jsonPayload))
	} else {
		reqBody = bytes.NewBuffer([]byte{})
	}

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request for %s: %w", fullURL, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Thread-safe token access
	c.authMutex.RLock()
	req.Header.Set("X-Auth-Token", c.authToken)
	c.authMutex.RUnlock()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request to %s failed: %w", fullURL, err)
	}
	defer resp.Body.Close()

	respBodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body from %s: %w", fullURL, readErr)
	}

	klog.Infof("Response Status: %s, Body: %s", resp.Status, string(respBodyBytes))

	// Enhanced error handling based on IBM Storage Virtualize API status codes
	if resp.StatusCode >= 400 {
		return respBodyBytes, resp.StatusCode, c.handleAPIError(resp.StatusCode, string(respBodyBytes), fullURL)
	}

	return respBodyBytes, resp.StatusCode, nil
}

// handleAPIError provides enhanced error handling for different HTTP status codes
func (c *FlashSystemAPIClient) handleAPIError(statusCode int, body, url string) error {
	switch statusCode {
	case http.StatusBadRequest: // 400
		return fmt.Errorf("bad request to %s: %s", url, body)
	case http.StatusUnauthorized: // 401
		return fmt.Errorf("unauthorized request to %s - check credentials: %s", url, body)
	case http.StatusForbidden: // 403
		return fmt.Errorf("forbidden request to %s - token expired or insufficient permissions: %s", url, body)
	case http.StatusNotFound: // 404
		return fmt.Errorf("resource not found at %s: %s", url, body)
	case http.StatusConflict: // 409
		return fmt.Errorf("conflict at %s - resource may already exist: %s", url, body)
	case http.StatusTooManyRequests: // 429
		return fmt.Errorf("too many requests to %s - rate limited: %s", url, body)
	case http.StatusInternalServerError: // 500
		return fmt.Errorf("internal server error at %s: %s", url, body)
	case http.StatusBadGateway: // 502
		return fmt.Errorf("bad gateway error at %s: %s", url, body)
	default:
		return fmt.Errorf("HTTP %d error at %s: %s", statusCode, url, body)
	}
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

// EnsureSingleHost creates or finds a single host with the given identifiers.
func (c *FlashSystemClonner) EnsureClonnerIgroup(hostName string, clonnerIdentifiers []string) (populator.MappingContext, error) {
	klog.Infof("Ensuring single host '%s' exists with identifiers: %v", hostName, clonnerIdentifiers)
	ctx := make(populator.MappingContext)

	if len(clonnerIdentifiers) == 0 {
		return nil, fmt.Errorf("no identifiers provided")
	}

	// Step 1: Categorize identifiers - separate FC WWPNs from iSCSI IQNs
	var fcWWPNs []string
	var iscsiIQNs []string

	for _, identifier := range clonnerIdentifiers {
		if strings.HasPrefix(identifier, "fc.") || isHexWWPN(identifier) {
			// It's a FC WWPN - extract and get the first half (non-virtual part)
			if strings.HasPrefix(identifier, "fc.") {
				wwpns := extractWWPNsFromFCFormat([]string{identifier})
				fcWWPNs = append(fcWWPNs, wwpns...)
			} else if isHexWWPN(identifier) {
				fcWWPNs = append(fcWWPNs, identifier)
			}
		} else {
			// Assume it's an iSCSI IQN
			iscsiIQNs = append(iscsiIQNs, identifier)
		}
	}

	klog.Infof("Categorized identifiers - FC WWPNs: %v, iSCSI IQNs: %v", fcWWPNs, iscsiIQNs)

	// Step 2: Check for existing hosts with any of these identifiers
	var existingHostName string
	var err error

	// Check FC WWPNs first (call once with all WWPNs)
	if len(fcWWPNs) > 0 {
		existingFCHosts, err := c.findAllHostsByWWPNs(fcWWPNs)
		if err != nil {
			klog.Warningf("Error searching for existing FC hosts: %v", err)
		} else if len(existingFCHosts) > 0 {
			existingHostName = existingFCHosts[0]
			klog.Infof("Found existing host '%s' with FC WWPNs %v", existingHostName, fcWWPNs)
		}
	}

	// Check iSCSI IQNs if no FC host found
	if existingHostName == "" && len(iscsiIQNs) > 0 {
		existingISCSIHosts, err := c.findAllHostsByIQNs(iscsiIQNs)
		if err != nil {
			klog.Warningf("Error searching for existing iSCSI hosts: %v", err)
		} else if len(existingISCSIHosts) > 0 {
			existingHostName = existingISCSIHosts[0]
			klog.Infof("Found existing host '%s' with iSCSI IQNs %v", existingHostName, iscsiIQNs)
		}
	}

	// Step 3: If existing host found, use it
	if existingHostName != "" {
		// Get the host details to find its ID
		hostDetails, err := c.getHostDetailsByName(existingHostName)
		if err != nil {
			return nil, fmt.Errorf("failed to get details for existing host '%s': %w", existingHostName, err)
		}

		ctx["host_id"] = hostDetails.ID
		ctx["host_name"] = hostDetails.Name
		klog.Infof("Using existing host '%s' with ID '%s'", hostDetails.Name, hostDetails.ID)
		return ctx, nil
	}

	// Step 4: No existing host found, create new host
	// Prioritize FC over iSCSI as per user requirements
	var newHostName string
	if len(fcWWPNs) > 0 {
		// Create FC host
		klog.Infof("Creating new FC host '%s' with WWPNs: %v", hostName, fcWWPNs)
		newHostName, err = c.createNewHost(hostName, fcWWPNs, true)
		if err != nil {
			return nil, fmt.Errorf("failed to create FC host: %w", err)
		}
	} else if len(iscsiIQNs) > 0 {
		// Create iSCSI host (only if no FC WWPNs exist)
		klog.Infof("Creating new iSCSI host '%s' with IQNs: %v", hostName, iscsiIQNs)
		newHostName, err = c.createNewHost(hostName, iscsiIQNs, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create iSCSI host: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no valid FC WWPNs or iSCSI IQNs found in identifiers: %v", clonnerIdentifiers)
	}

	// Get the details of the newly created host
	hostDetails, err := c.getHostDetailsByName(newHostName)
	if err != nil {
		return nil, fmt.Errorf("failed to get details for newly created host '%s': %w", newHostName, err)
	}

	ctx["host_id"] = hostDetails.ID
	ctx["host_name"] = hostDetails.Name
	ctx["created_host"] = true // Flag to indicate we created this host for cleanup
	klog.Infof("Successfully created new host '%s' with ID '%s'", hostDetails.Name, hostDetails.ID)
	return ctx, nil
}

// createNewHost creates a new host without checking for existing ones (used when we've already determined none exist)
func (c *FlashSystemClonner) createNewHost(hostName string, identifiers []string, isFibreChannel bool) (string, error) {
	// Check if our desired host name exists and adjust if needed
	filterPayload := map[string]string{
		"filtervalue": fmt.Sprintf("name=%s", hostName),
	}

	hostBytes, hostStatus, hostErr := c.api.makeRequest("POST", "/lshost", filterPayload)
	if hostErr != nil {
		return "", fmt.Errorf("failed to query host by name: %w", hostErr)
	}

	if hostStatus == http.StatusOK {
		var existingHosts []FlashSystemHost
		if err := json.Unmarshal(hostBytes, &existingHosts); err == nil && len(existingHosts) > 0 {
			// Generate a unique name by appending a suffix
			hostName = fmt.Sprintf("%s-alt", hostName)
			klog.Infof("Host name conflict, using alternative name: %s", hostName)
		}
	}

	// Create new host with unique WWPNs and name
	klog.Infof("Creating NEW host '%s' with identifiers: %v (FC: %t)", hostName, identifiers, isFibreChannel)

	createPayload := map[string]interface{}{
		"name": hostName,
	}

	if isFibreChannel {
		// Fibre Channel host
		wwpnString := strings.Join(identifiers, ":")
		createPayload["fcwwpn"] = wwpnString
		createPayload["force"] = true
		createPayload["protocol"] = "fcscsi"
		createPayload["type"] = "generic"
		klog.Infof("Creating FC host '%s' with WWPNs: %s", hostName, wwpnString)
	} else {
		// iSCSI host
		iqnString := strings.Join(identifiers, ",")
		createPayload["iscsiname"] = iqnString
		createPayload["protocol"] = "iscsi"
		createPayload["type"] = "generic"
		klog.Infof("Creating iSCSI host '%s' with IQNs: %s", hostName, iqnString)
	}

	// Log the exact payload for debugging
	if payloadJSON, err := json.MarshalIndent(createPayload, "", "  "); err == nil {
		klog.Infof("FlashSystem mkhost API request payload: %s", string(payloadJSON))
	}

	// Make the mkhost API call
	respBytes, respStatus, err := c.api.makeRequest("POST", "/mkhost", createPayload)
	if err != nil {
		return "", fmt.Errorf("failed to create host: %w", err)
	}

	if respStatus != http.StatusOK && respStatus != http.StatusCreated {
		return "", fmt.Errorf("failed to create host: status %d, body: %s", respStatus, string(respBytes))
	}

	klog.Infof("Successfully created host '%s'", hostName)
	return hostName, nil
}

// Map maps a VDisk to a Host using mkvolumehostmap API.
func (c *FlashSystemClonner) Map(initiatorGroup string, targetLUN populator.LUN, context populator.MappingContext) (populator.LUN, error) {
	hostID, ok := context["host_id"].(string)
	if !ok || hostID == "" {
		return populator.LUN{}, fmt.Errorf("host_id not found or invalid in mapping context for initiator group '%s'", initiatorGroup)
	}

	vdiskID := targetLUN.Name // The LUN.Name field should hold the VDisk ID for this provider.
	klog.Infof("Mapping LUN (VDisk ID '%s') to Host '%s' (Host ID '%s')", vdiskID, initiatorGroup, hostID)

	// Create the mapping using mkvdiskhostmap API endpoint
	mapPayload := map[string]string{
		"host": hostID,
	}

	endpoint := fmt.Sprintf("/mkvdiskhostmap/%s", vdiskID)
	mapBody, mapStatus, mapErr := c.api.makeRequest("POST", endpoint, mapPayload)

	// Handle the specific case where mapping already exists
	if mapErr != nil && mapStatus == http.StatusConflict && strings.Contains(mapErr.Error(), "CMMVC5878E") {
		klog.Infof("VDisk '%s' is already mapped to Host '%s', continuing...", vdiskID, hostID)
	} else if mapErr != nil {
		return populator.LUN{}, fmt.Errorf("failed to create host mapping: %w", mapErr)
	} else if mapStatus != http.StatusOK && mapStatus != http.StatusCreated {
		return populator.LUN{}, fmt.Errorf("failed to map VDisk '%s' to Host '%s', status: %d, body: %s", vdiskID, hostID, mapStatus, string(mapBody))
	}

	klog.Infof("Successfully created mapping of VDisk '%s' to Host '%s'.", vdiskID, hostID)

	return targetLUN, nil
}

// UnMap removes a VDisk mapping from a Host.
func (c *FlashSystemClonner) UnMap(initiatorGroup string, targetLUN populator.LUN, context populator.MappingContext) error {
	hostID, ok := context["host_id"].(string)
	if !ok || hostID == "" {
		return fmt.Errorf("host_id not found or invalid in mapping context for initiator group '%s'", initiatorGroup)
	}

	vdiskID := targetLUN.Name // VDisk ID from the LUN object.
	klog.Infof("Unmapping LUN (VDisk ID '%s') from Host '%s' (Host ID '%s')", vdiskID, initiatorGroup, hostID)

	// Use v1 API endpoint for removing host mapping
	payload := map[string]string{
		"host": hostID,
	}

	endpoint := fmt.Sprintf("/rmvdiskhostmap/%s", vdiskID)
	unmapBody, unmapStatus, unmapErr := c.api.makeRequest("POST", endpoint, payload)
	if unmapErr != nil {
		return fmt.Errorf("failed to unmap VDisk from host: %w", unmapErr)
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

	// Clean up the host if we created it and if it has no more mappings
	createdHost, _ := context["created_host"].(bool)
	if createdHost {
		c.cleanupEmptyHost(hostID, initiatorGroup)
	}

	return nil
}

// cleanupEmptyHost removes a host if it has no mappings (safe cleanup)
func (c *FlashSystemClonner) cleanupEmptyHost(hostID, hostName string) {
	// Check if host has any remaining mappings
	filterPayload := map[string]string{
		"filtervalue": fmt.Sprintf("host_id=%s", hostID),
	}

	mappingsBytes, status, err := c.api.makeRequest("POST", "/lshostvdiskmap", filterPayload)
	if err != nil {
		klog.Warningf("Failed to check host mappings before cleanup: %v", err)
		return
	}

	if status == http.StatusOK {
		var mappings []FlashSystemVolumeHostMapping
		if err := json.Unmarshal(mappingsBytes, &mappings); err == nil && len(mappings) > 0 {
			klog.Infof("Host '%s' still has %d mappings, not cleaning up", hostName, len(mappings))
			return
		}
	}

	// Host has no mappings, safe to remove it
	klog.Infof("Cleaning up empty host '%s' (ID: %s)", hostName, hostID)

	rmPayload := map[string]string{
		"host": hostID,
	}

	rmBody, rmStatus, rmErr := c.api.makeRequest("POST", "/rmhost", rmPayload)
	if rmErr != nil {
		klog.Warningf("Failed to cleanup host: %v", rmErr)
		return
	}

	if rmStatus != http.StatusOK && rmStatus != http.StatusNoContent {
		klog.Warningf("Failed to cleanup host '%s', status: %d, body: %s", hostName, rmStatus, string(rmBody))
		return
	}

	klog.Infof("Successfully cleaned up empty host '%s'", hostName)
}

// CurrentMappedGroups returns the host names a VDisk is mapped to.
func (c *FlashSystemClonner) CurrentMappedGroups(targetLUN populator.LUN, context populator.MappingContext) ([]string, error) {
	vdiskID := targetLUN.Name // VDisk ID is stored in the Name field.
	klog.Infof("Getting current mapped groups for LUN (VDisk ID '%s')", vdiskID)

	groupSet := make(map[string]bool)
	uniqueGroups := []string{}

	// Check host mappings using lsvdiskhostmap with vdisk_id in the URL path
	endpoint := fmt.Sprintf("/lsvdiskhostmap/%s", vdiskID)
	hostBodyBytes, hostStatus, hostErr := c.api.makeRequest("POST", endpoint, map[string]string{})
	if hostErr != nil {
		return nil, fmt.Errorf("failed to get host mappings: %w", hostErr)
	}

	if hostStatus != http.StatusOK {
		return nil, fmt.Errorf("failed to get host mappings, status: %d, body: %s", hostStatus, string(hostBodyBytes))
	}

	var hostMappings []FlashSystemVolumeHostMapping
	if err := json.Unmarshal(hostBodyBytes, &hostMappings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal host mappings: %w. Body: %s", err, string(hostBodyBytes))
	}

	for _, m := range hostMappings {
		if !groupSet[m.HostName] {
			groupSet[m.HostName] = true
			uniqueGroups = append(uniqueGroups, m.HostName)
			klog.Infof("Found host mapping: %s", m.HostName)
		}
	}

	klog.Infof("LUN (VDisk ID '%s') is mapped to host groups: %v", vdiskID, uniqueGroups)
	return uniqueGroups, nil
}

// createLUNFromVDisk creates a LUN object from a FlashSystemVolume
func (c *FlashSystemClonner) createLUNFromVDisk(vdiskDetails FlashSystemVolume, volumeHandle string) (populator.LUN, error) {
	vdiskUID := strings.ToLower(vdiskDetails.VdiskUID)
	if vdiskUID == "" {
		return populator.LUN{}, fmt.Errorf("resolved volume '%s' has an empty UID", vdiskDetails.Name)
	}

	// FlashSystem vdiskUID already contains the full NAA identifier including the IBM vendor prefix

	naaDeviceID := "naa." + vdiskUID

	lun := populator.LUN{
		Name:         vdiskDetails.ID,
		VolumeHandle: vdiskDetails.Name,
		SerialNumber: vdiskDetails.VdiskUID,
		NAA:          naaDeviceID,
	}

	klog.Infof("Resolved volume handle '%s' to LUN: Name(ID)=%s, SN(UID)=%s, NAA=%s, VDisk Name=%s",
		volumeHandle, lun.Name, lun.SerialNumber, lun.NAA, vdiskDetails.Name)
	return lun, nil
}

// ResolvePVToLUN resolves a PersistentVolume to a LUN by finding a volume that has the PV name as a suffix.
func (c *FlashSystemClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	klog.Infof("Resolving PersistentVolume '%s' to LUN details", pv.Name)

	// Get all vdisks and search by suffix
	allVDisksBytes, allVDisksStatus, allVDisksErr := c.api.makeRequest("POST", "/lsvdisk", map[string]string{})
	if allVDisksErr != nil {
		return populator.LUN{}, fmt.Errorf("failed to get vdisks: %w", allVDisksErr)
	}
	if allVDisksStatus != http.StatusOK {
		return populator.LUN{}, fmt.Errorf("failed to get vdisks, status: %d, body: %s", allVDisksStatus, string(allVDisksBytes))
	}

	var allVDisks []FlashSystemVolume
	if err := json.Unmarshal(allVDisksBytes, &allVDisks); err != nil {
		return populator.LUN{}, fmt.Errorf("failed to unmarshal vdisks response: %w. Body: %s", err, string(allVDisksBytes))
	}

	for _, vdisk := range allVDisks {
		if strings.HasSuffix(vdisk.Name, pv.Name) {
			klog.Infof("Found matching volume: '%s' for PV '%s'", vdisk.Name, pv.Name)
			return c.createLUNFromVDisk(vdisk, pv.VolumeHandle)
		}
	}

	return populator.LUN{}, fmt.Errorf("volume with PV name '%s' as suffix not found", pv.Name)
}

// getHostPorts gets host ports directly from the API
func (c *FlashSystemClonner) getHostPorts() ([]HostPort, error) {
	klog.Infof("Fetching host ports using lshostports")
	hostPortBytes, status, err := c.api.makeRequest("POST", "/lshostports", map[string]string{})
	if err != nil || status != http.StatusOK {
		return nil, fmt.Errorf("failed to list host ports: %w, status: %d", err, status)
	}

	// Parse response as host ports
	var hostPorts []HostPort
	if err := json.Unmarshal(hostPortBytes, &hostPorts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal host ports: %w", err)
	}

	klog.Infof("Retrieved %d host ports from lshostports", len(hostPorts))
	return hostPorts, nil
}

// findAllHostsByIdentifiers searches for hosts using host port discovery - unified function for both WWPNs and IQNs
func (c *FlashSystemClonner) findAllHostsByIdentifiers(identifiers []string, identifierType string) ([]string, error) {
	if len(identifiers) == 0 {
		return nil, nil
	}

	klog.Infof("Searching for hosts with %s using host port discovery: %v", identifierType, identifiers)

	// Get host ports
	hostPorts, err := c.getHostPorts()
	if err != nil {
		return nil, err
	}

	foundHosts := make(map[string]bool) // Use map to avoid duplicates
	var hostNames []string

	// Normalize identifiers for comparison (just lowercase)
	normalizedIdentifiers := make(map[string]string) // normalized -> original mapping
	for _, identifier := range identifiers {
		normalized := strings.ToLower(identifier)
		normalizedIdentifiers[normalized] = identifier
		klog.V(4).Infof("Normalized %s: %s -> %s", identifierType, identifier, normalized)
	}

	// Search through host ports for matching identifiers
	for _, port := range hostPorts {
		// Both WWPNs and IQNs can be in the WWPN field according to user, but also check IQN field for completeness
		fieldsToCheck := []string{port.WWPN, port.IQN}

		for _, fieldValue := range fieldsToCheck {
			if fieldValue != "" {
				normalizedFieldValue := strings.ToLower(fieldValue)
				klog.V(4).Infof("Checking field value: %s (normalized: %s) for host: %s", fieldValue, normalizedFieldValue, port.HostName)

				// Check if normalized value matches any of our target identifiers
				if originalIdentifier, exists := normalizedIdentifiers[normalizedFieldValue]; exists {
					if !foundHosts[port.HostName] {
						klog.Infof("Found host '%s' for %s %s (port field value: %s) via host port", port.HostName, identifierType, originalIdentifier, fieldValue)
						foundHosts[port.HostName] = true
						hostNames = append(hostNames, port.HostName)
					}
				}
			}
		}
	}

	klog.Infof("Found %d existing hosts via host port discovery: %v", len(hostNames), hostNames)
	return hostNames, nil
}

// findAllHostsByWWPNs searches for hosts with WWPNs using the unified function
func (c *FlashSystemClonner) findAllHostsByWWPNs(wwpns []string) ([]string, error) {
	return c.findAllHostsByIdentifiers(wwpns, "WWPNs")
}

// findAllHostsByIQNs searches for hosts with IQNs using the unified function
func (c *FlashSystemClonner) findAllHostsByIQNs(iqns []string) ([]string, error) {
	return c.findAllHostsByIdentifiers(iqns, "IQNs")
}

// getHostDetailsByName gets detailed information about a specific host and returns FlashSystemHost
func (c *FlashSystemClonner) getHostDetailsByName(hostName string) (*FlashSystemHost, error) {
	filterPayload := map[string]string{
		"filtervalue": fmt.Sprintf("name=%s", hostName),
	}

	hostBytes, status, err := c.api.makeRequest("POST", "/lshost", filterPayload)
	if err != nil || status != http.StatusOK {
		return nil, fmt.Errorf("failed to get host details for %s: %w, status: %d", hostName, err, status)
	}

	var hosts []FlashSystemHost
	if err := json.Unmarshal(hostBytes, &hosts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal host details: %w", err)
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("host %s not found", hostName)
	}

	return &hosts[0], nil
}
