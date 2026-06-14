package primera3par

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/fcutil"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"k8s.io/klog/v2"
)

// SystemInfo represents the response from GET /api/v1/system
type SystemInfo struct {
	SystemVersion string `json:"systemVersion"`
	Model         string `json:"model"`
}

type Primera3ParClient interface {
	GetSessionKey() (string, error)
	EnsureLunMapped(initiatorGroup string, targetLUN populator.LUN) (populator.LUN, error)
	LunUnmap(ctx context.Context, initiatorGroupName string, lunName string) error
	EnsureHostsWithIds(adapterIds []string) ([]string, error)
	EnsureHostSetExists(hostSetName string) error
	AddHostToHostSet(hostSetName string, hostName string) error
	GetLunDetailsByVolumeName(lunName string, lun populator.LUN) (populator.LUN, error)
	CurrentMappedGroups(volumeName string, mappingContext populator.MappingContext) ([]string, error)
	CopyVolume(sourceVolName string, destVolName string, progress chan<- uint64) error
	GetVolumes(query string) ([]Volume, error)
	GetSystemInfo() (SystemInfo, error)
}

type Volume struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	WWN     string `json:"wwn"`
	UserCPG string `json:"userCPG"`
	SnapCPG string `json:"snapCPG"`
}

type HostsResponse struct {
	Total   int    `json:"total"`
	Members []Host `json:"members"`
}

type Host struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	Descriptors Descriptor  `json:"descriptors"`
	FCPaths     []FCPath    `json:"FCPaths"`
	ISCSIPaths  []ISCSIPath `json:"iSCSIPaths"`
	Persona     int         `json:"persona"`
	Links       []Link      `json:"links"`
}

type Descriptor struct {
	OS string `json:"os"`
}

type FCPath struct {
	WWN string `json:"wwn"`
}

type ISCSIPath struct {
	Name      string `json:"name"`
	IPAddr    string `json:"IPAddr"`
	HostSpeed int    `json:"hostSpeed"`
}

type Link struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

type Primera3ParClientWsImpl struct {
	BaseURL          string
	SessionKey       string
	Password         string
	Username         string
	HTTPClient       *http.Client
	SessionStartTime time.Time
	wsApiBuild       int
}

func NewPrimera3ParClientWsImpl(storageHostname, storageUsername, storagePassword string, skipSSLVerification bool) Primera3ParClientWsImpl {
	return Primera3ParClientWsImpl{
		BaseURL:  storageHostname,
		Password: storagePassword,
		Username: storageUsername,
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: skipSSLVerification}, // Disable SSL verification
			},
		},
	}
}

// EnsureHostsWithIds We return a list of host names that are connected to the adapters provided. If a host already exists we find it,
// if it does not, we crate a new one. When we create a new host it will always have one path, but an existing host may
// aggregate several.
func (p *Primera3ParClientWsImpl) EnsureHostsWithIds(adapterIds []string) ([]string, error) {
	hostnames := make([]string, len(adapterIds))
	for _, adapterId := range adapterIds {
		hostName, err := p.getHostByAdapterId(adapterId)
		if err != nil {
			return nil, fmt.Errorf("failed to get host by adapterId: %w", err)
		}
		if hostName != "" {
			hostnames = append(hostnames, hostName)
			continue
		}
		hostName = uuid.New().String()
		hostName = hostName[:10]
		err = p.createHost(hostName, adapterId)
		if err != nil {
			return nil, err
		}

		hostnames = append(hostnames, hostName)
	}
	hostnames = cleanHostnames(hostnames)
	return hostnames, nil
}

func cleanHostnames(hosts []string) []string {
	seen := make(map[string]struct{}, len(hosts))
	var out []string
	for _, h := range hosts {
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	return out
}

func (p *Primera3ParClientWsImpl) getHostByAdapterId(id string) (string, error) {
	var rawFilter string
	var targetWWPN string
	isFC := strings.HasPrefix(id, "fc.")
	isISCSI := strings.HasPrefix(id, "iqn.")

	if isFC {
		_, wwpn, err := fcutil.ParseFCAdapter(id)
		if err != nil {
			return "", fmt.Errorf("invalid FC adapter id %q: %w", id, err)
		}
		targetWWPN = fcutil.NormalizeWWN(wwpn)
		rawFilter = fmt.Sprintf(`" FCPaths[wwn EQ %s] "`, targetWWPN)
	} else if isISCSI {
		rawFilter = fmt.Sprintf(`" iSCSIPaths[name EQ %s] "`, id)
	} else {
		klog.Infof("host with adapterId %s not found since this adapter type is not supported", id)
		return "", nil
	}

	// Try server-side filter first (fast path, not supported on all firmware)
	esc := url.PathEscape(rawFilter)
	uri := fmt.Sprintf("%s/api/v1/hosts?query=%s", p.BaseURL, esc)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	var respData HostsResponse
	if err := p.doRequestUnmarshalResponse(req, "getHostByAdapterId", &respData); err == nil && len(respData.Members) > 0 {
		return respData.Members[0].Name, nil
	}

	// Fall back to client-side matching (works on all firmware versions)
	klog.Infof("server-side host filter failed or returned empty, falling back to client-side matching for %s", id)
	return p.getHostByAdapterIdClientSide(id, isFC, targetWWPN)
}

func (p *Primera3ParClientWsImpl) getHostByAdapterIdClientSide(id string, isFC bool, targetWWPN string) (string, error) {
	uri := fmt.Sprintf("%s/api/v1/hosts", p.BaseURL)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	var respData HostsResponse
	if err := p.doRequestUnmarshalResponse(req, "getHostByAdapterId", &respData); err != nil {
		return "", err
	}

	for _, host := range respData.Members {
		if isFC {
			for _, fc := range host.FCPaths {
				if fcutil.NormalizeWWN(fc.WWN) == targetWWPN {
					return host.Name, nil
				}
			}
		} else {
			for _, iscsi := range host.ISCSIPaths {
				if iscsi.Name == id {
					return host.Name, nil
				}
			}
		}
	}
	return "", nil
}

func (p *Primera3ParClientWsImpl) hostExists(hostname string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/hosts/%s", p.BaseURL, url.PathEscape(hostname))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.doRequest(req, "hostExists")
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode, string(body))
}

func (p *Primera3ParClientWsImpl) createHost(hostname, adapterId string) error {
	url := fmt.Sprintf("%s/api/v1/hosts", p.BaseURL)

	body := map[string]interface{}{
		"name":    hostname,
		"persona": 11,
	}

	if strings.HasPrefix(adapterId, "fc.") {
		_, wwpn, err := fcutil.ParseFCAdapter(adapterId)
		if err != nil {
			return fmt.Errorf("invalid FC adapter id %q: %w", adapterId, err)
		}
		body["FCWWNs"] = []string{fcutil.NormalizeWWN(wwpn)}
	} else {
		body["iSCSINames"] = []string{adapterId}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal create-host body: %w", err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create POST request: %w", err)
	}

	resp, err := p.doRequest(req, "createHost")
	if err != nil {
		return fmt.Errorf("createHost request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("createHost returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
func sanitizeWWN(raw string) string {
	return fcutil.NormalizeWWN(raw)
}

func (p *Primera3ParClientWsImpl) GetSessionKey() (string, error) {
	if time.Since(p.SessionStartTime) < 3*time.Minute && p.SessionKey != "" {
		return p.SessionKey, nil
	}
	url := fmt.Sprintf("%s/api/v1/credentials", p.BaseURL)

	requestBody := map[string]string{
		"user":     p.Username,
		"password": p.Password,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to encode JSON: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		var errorResp struct {
			Code int    `json:"code"`
			Desc string `json:"desc"`
		}

		if err := json.Unmarshal(bodyBytes, &errorResp); err == nil {
			return "", fmt.Errorf("authentication failed: %s (code %d)", errorResp.Desc, errorResp.Code)
		}
		return "", fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response map[string]string
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return "", fmt.Errorf("failed to parse session key response: %w", err)
	}

	if sessionKey, ok := response["key"]; ok {
		p.SessionKey = sessionKey
		p.SessionStartTime = time.Now()
		klog.Info("Successfully obtained new session key")
		return sessionKey, nil
	}

	return "", fmt.Errorf("failed to retrieve session key, response: %s", string(bodyBytes))
}

func ensureSetPrefix(name string) string {
	if strings.HasPrefix(name, "set:") {
		return name
	}
	return fmt.Sprintf("set:%s", name)
}

func (p *Primera3ParClientWsImpl) EnsureLunMapped(initiatorGroup string, targetLUN populator.LUN) (populator.LUN, error) {
	targetLUN.IQN = initiatorGroup
	hostSetName := ensureSetPrefix(initiatorGroup)
	vlun, err := p.GetVLun(targetLUN.Name, hostSetName)
	if err != nil {
		return populator.LUN{}, err
	}

	if vlun != nil {
		return targetLUN, nil
	}

	lunID, err := p.GetFreeLunID(initiatorGroup)
	if err != nil {
		return populator.LUN{}, err
	}

	// note autoLun is on, and lun is set as well - this combination works for both primera and 3par.
	// "autoLun" alone fails for 3par despite documentation, and setting lun fails for primera.
	requestBody := map[string]interface{}{
		"volumeName": targetLUN.Name,
		"lun":        lunID,
		"hostname":   hostSetName,
		"autoLun":    true,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to encode JSON: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/vluns", p.BaseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.doRequest(req, "ensureLunMapping")
	if err != nil {
		return populator.LUN{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return populator.LUN{}, fmt.Errorf("failed to map LUN: status %d, resp: %v", resp.StatusCode, resp)
	}

	return targetLUN, nil
}

func (p *Primera3ParClientWsImpl) LunUnmap(ctx context.Context, initiatorGroupName string, lunName string) error {
	lunID, err := p.GetVLunID(lunName, ensureSetPrefix(initiatorGroupName))
	if err != nil {
		return fmt.Errorf("failed to get LUN ID: %w", err)
	}

	fields := map[string]interface{}{
		"LUN":         lunName,
		"igroup":      initiatorGroupName,
		"LUN ID Used": lunID,
	}

	log.Printf("LunUnmap: %v", fields)

	url := fmt.Sprintf("%s/api/v1/vluns/%s,%d,%s", p.BaseURL, lunName, lunID, ensureSetPrefix(initiatorGroupName))

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	p.setReqHeadersWithSessionKey(req)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	resp, err = p.handleUnauthorizedSessionKey(resp, req, err)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to unmap LUN: status %d", resp.StatusCode)
	}

	log.Printf("LunUnmap: Successfully unmapped LUN %s from %s", lunName, initiatorGroupName)
	return nil
}

func (p *Primera3ParClientWsImpl) GetFreeLunID(initiatorGroupName string) (int, error) {
	url := fmt.Sprintf("%s/api/v1/vluns", p.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	var response struct {
		Members []struct {
			LUN      int    `json:"lun"`
			Hostname string `json:"hostname"`
		} `json:"members"`
	}
	err = p.doRequestUnmarshalResponse(req, "getFreeLunId", &response)
	if err != nil {
		return 0, err
	}

	usedLUNs := make(map[int]bool)
	hostSetName := ensureSetPrefix(initiatorGroupName)
	for _, vlun := range response.Members {
		if vlun.Hostname == hostSetName {
			usedLUNs[vlun.LUN] = true
		}
	}

	for i := 1; i <= 255; i++ {
		if !usedLUNs[i] {
			return i, nil
		}
	}

	return 0, fmt.Errorf("no available LUN ID found for host %s", initiatorGroupName)
}

func (p *Primera3ParClientWsImpl) GetVLunSerial(volumeName, hostName string) (string, error) {
	lun, err := p.GetVLun(volumeName, hostName)
	if err != nil {
		return "", err
	}
	if lun == nil {
		return "", fmt.Errorf("LUN not found for volume %s and host %s at GetVLunSerial", volumeName, hostName)
	}
	return lun.Serial, nil
}

type VLun struct {
	VolumeName string `json:"volumeName"`
	LUN        int    `json:"lun"`
	Hostname   string `json:"hostname"`
	Serial     string `json:"serial"`
}

func (p *Primera3ParClientWsImpl) GetVLun(volumeName, hostname string) (*VLun, error) {
	url := fmt.Sprintf("%s/api/v1/vluns", p.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var response struct {
		Members []VLun `json:"members"`
	}

	err = p.doRequestUnmarshalResponse(req, "getVLun", &response)
	if err != nil {
		return nil, err
	}
	for _, vlun := range response.Members {
		if vlun.VolumeName == volumeName && vlun.Hostname == hostname {
			return &vlun, nil
		}
	}
	return nil, nil
}

func (p *Primera3ParClientWsImpl) GetVLunID(lunName, initiatorGroupName string) (int, error) {
	lun, err := p.GetVLun(lunName, initiatorGroupName)
	if err != nil {
		return 0, err
	}
	if lun == nil {
		return 0, fmt.Errorf("LUN not found for volume %s and host %s, at GetVLunID", lunName, initiatorGroupName)
	}
	return lun.LUN, nil
}

func (p *Primera3ParClientWsImpl) GetLunDetailsByVolumeName(volumeName string, lun populator.LUN) (populator.LUN, error) {
	cutVolName := prefixOfString(volumeName, 31)
	url := fmt.Sprintf("%s/api/v1/volumes/%s", p.BaseURL, url.PathEscape(cutVolName))

	reqType := "getVolume"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to create request: %w", err)
	}
	type MyResponse struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
		WWN  string `json:"wwn"`
	}

	var response MyResponse

	err = p.doRequestUnmarshalResponse(req, reqType, &response)
	if err != nil {
		return populator.LUN{}, err
	}

	if response.Name != "" {
		lun.Name = cutVolName
		lun.NAA = fmt.Sprintf("naa.%s", strings.ToLower(response.WWN))
		return lun, nil
	}
	return populator.LUN{}, fmt.Errorf("volume not found for volume: %s", cutVolName)
}

func (p *Primera3ParClientWsImpl) CurrentMappedGroups(volumeName string, mappingContext populator.MappingContext) ([]string, error) {
	type VLUN struct {
		LUN        int    `json:"lun"`
		VolumeName string `json:"volumeName"`
		Hostname   string `json:"hostname"`
	}

	type Response struct {
		Members []VLUN `json:"members"`
	}

	var response Response

	url := fmt.Sprintf("%s/api/v1/vluns", p.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []string{}, fmt.Errorf("failed to create request: %w", err)
	}
	err = p.doRequestUnmarshalResponse(req, "GET", &response)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VLUNs: %w", err)
	}

	hostnameSet := make(map[string]struct{})

	for _, vlun := range response.Members {
		if vlun.VolumeName == volumeName {
			hostnameSet[vlun.Hostname] = struct{}{}
		}
	}

	hostnames := make([]string, 0, len(hostnameSet))
	for hostname := range hostnameSet {
		hostnames = append(hostnames, hostname)
	}

	return hostnames, nil
}

func (p *Primera3ParClientWsImpl) doRequest(req *http.Request, reqDescription string) (*http.Response, error) {
	_, err := p.GetSessionKey()
	if err != nil {
		return nil, err
	}

	p.setReqHeadersWithSessionKey(req)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed for %s: %w", reqDescription, err)
	}

	if resp, err = p.handleUnauthorizedSessionKey(resp, req, err); err != nil {
		return nil, fmt.Errorf("failed for %s: %w", reqDescription, err)
	}

	return resp, nil
}

func (p *Primera3ParClientWsImpl) doRequestUnmarshalResponse(req *http.Request, reqDescription string, response interface{}) error {
	_, err := p.GetSessionKey()
	if err != nil {
		return err
	}

	p.setReqHeadersWithSessionKey(req)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed for %s: %w", reqDescription, err)
	}
	defer resp.Body.Close()

	if resp, err = p.handleUnauthorizedSessionKey(resp, req, err); err != nil {
		return fmt.Errorf("failed for %s: %w", reqDescription, err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed for %s: status %d, body: %s", reqDescription, resp.StatusCode, string(body))
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response for %s: %w", reqDescription, err)
	}

	if err := json.Unmarshal(bodyBytes, response); err != nil {
		return fmt.Errorf("failed to parse JSON for %s: %w", reqDescription, err)
	}

	return nil
}

func (p *Primera3ParClientWsImpl) handleUnauthorizedSessionKey(resp *http.Response, req *http.Request, err error) (*http.Response, error) {
	if resp.StatusCode == http.StatusUnauthorized {
		if _, err := p.GetSessionKey(); err != nil {
			return nil, fmt.Errorf("failed to refresh session key: %w", err)
		}

		p.setReqHeadersWithSessionKey(req)
		resp, err = p.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("retry request failed: %w", err)
		}
		defer resp.Body.Close()
	}
	return resp, nil
}

func (p *Primera3ParClientWsImpl) EnsureHostSetExists(hostSetName string) error {
	url := fmt.Sprintf("%s/api/v1/hostsets/%s", p.BaseURL, url.PathEscape(hostSetName))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := p.doRequest(req, "ensureHostSetExists, find set")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil // Host set already exists
	}

	createURL := fmt.Sprintf("%s/api/v1/hostsets", p.BaseURL)
	requestBody := map[string]interface{}{
		"name": hostSetName,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	req, err = http.NewRequest("POST", createURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	respCreate, err := p.doRequest(req, "EnsuresHostSetExists")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer respCreate.Body.Close()

	if respCreate.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(respCreate.Body)
		return fmt.Errorf("failed to create host set: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (p *Primera3ParClientWsImpl) setReqHeadersWithSessionKey(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HP3PAR-WSAPI-SessionKey", p.SessionKey)
}

func (p *Primera3ParClientWsImpl) AddHostToHostSet(hostSetName string, hostName string) error {
	url := fmt.Sprintf("%s/api/v1/hostsets/%s", p.BaseURL, url.PathEscape(hostSetName))

	requestBody := map[string]interface{}{
		"action": 1,
		"setmembers": []string{
			hostName,
		},
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.doRequest(req, "AddHostToHostSet")

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)

	var apiError struct {
		Code int    `json:"code"`
		Desc string `json:"desc"`
	}

	if err := json.Unmarshal(body, &apiError); err == nil {
		if apiError.Code == 104 {
			klog.Errorf("Host %s is already in host set %s. Ignoring duplicate.\n", hostName, hostSetName)
			return nil
		}
	}
	return fmt.Errorf("failed to add host to host set: status %d, body: %s", resp.StatusCode, string(body))
}

func prefixOfString(s string, length int) string {
	runes := []rune(s)
	if len(runes) > length {
		return string(runes[:length])
	}
	return s
}

func (p *Primera3ParClientWsImpl) GetVolume(volumeName string) (Volume, error) {
	reqURL := fmt.Sprintf("%s/api/v1/volumes/%s", p.BaseURL, url.PathEscape(volumeName))

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return Volume{}, fmt.Errorf("failed to create GetVolume request: %w", err)
	}

	var vol Volume
	if err := p.doRequestUnmarshalResponse(req, "GetVolume", &vol); err != nil {
		return Volume{}, fmt.Errorf("failed to get volume %s: %w", volumeName, err)
	}

	return vol, nil
}

func (p *Primera3ParClientWsImpl) renameVolume(oldName, newName string) error {
	reqURL := fmt.Sprintf("%s/api/v1/volumes/%s", p.BaseURL, url.PathEscape(oldName))

	body, err := json.Marshal(map[string]string{"newName": newName})
	if err != nil {
		return fmt.Errorf("failed to encode rename request: %w", err)
	}

	req, err := http.NewRequest("PUT", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create rename request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.doRequest(req, "renameVolume")
	if err != nil {
		return fmt.Errorf("renameVolume failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("renameVolume failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	klog.Infof("Renamed volume %s -> %s", oldName, newName)
	return nil
}

func (p *Primera3ParClientWsImpl) setVolumeSnapCPG(volumeName, snapCPG string) error {
	reqURL := fmt.Sprintf("%s/api/v1/volumes/%s", p.BaseURL, url.PathEscape(volumeName))

	body, err := json.Marshal(map[string]string{"snapCPG": snapCPG})
	if err != nil {
		return fmt.Errorf("failed to encode setVolumeSnapCPG request: %w", err)
	}

	req, err := http.NewRequest("PUT", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create setVolumeSnapCPG request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.doRequest(req, "setVolumeSnapCPG")
	if err != nil {
		return fmt.Errorf("setVolumeSnapCPG failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("setVolumeSnapCPG failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	klog.Infof("Set snapCPG=%s on volume %s", snapCPG, volumeName)
	return nil
}

func (p *Primera3ParClientWsImpl) deleteVolume(volumeName string) error {
	reqURL := fmt.Sprintf("%s/api/v1/volumes/%s", p.BaseURL, url.PathEscape(volumeName))

	req, err := http.NewRequest("DELETE", reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create deleteVolume request: %w", err)
	}

	resp, err := p.doRequest(req, "deleteVolume")
	if err != nil {
		return fmt.Errorf("deleteVolume failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deleteVolume failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	klog.Infof("Deleted volume %s", volumeName)
	return nil
}

func (p *Primera3ParClientWsImpl) CopyVolume(sourceVolName string, destVolName string, progress chan<- uint64) error {
	sourceName := prefixOfString(sourceVolName, 31)
	destName := prefixOfString(destVolName, 31)

	sourceVol, err := p.GetVolume(sourceName)
	if err != nil {
		return fmt.Errorf("failed to get source volume details: %w", err)
	}
	klog.Infof("Source volume: name=%s id=%d wwn=%s cpg=%s snapCPG=%s", sourceVol.Name, sourceVol.Id, sourceVol.WWN, sourceVol.UserCPG, sourceVol.SnapCPG)

	apiBuild, err := p.getWsApiBuild()
	if err != nil {
		return fmt.Errorf("failed to query WSAPI version: %w", err)
	}

	if sourceVol.SnapCPG == "" && apiBuild < wsApiBuild2023 {
		snapCPG := sourceVol.UserCPG
		klog.Infof("Source volume %s has no snapCPG, setting it to %s", sourceName, snapCPG)
		if err := p.setVolumeSnapCPG(sourceName, snapCPG); err != nil {
			return fmt.Errorf("failed to set snapCPG on source volume %s: %w", sourceName, err)
		}
	} else if sourceVol.SnapCPG == "" {
		klog.Infof("Source volume %s has no snapCPG but WSAPI build %d >= %d — array manages snap space at pool level", sourceName, apiBuild, wsApiBuild2023)
	}

	// Rename the CSI-provisioned dest volume aside so we can create a snapshot
	// with the same name. The CSI driver locates volumes by name, so the
	// snapshot seamlessly takes its place after promotion.
	tempName := prefixOfString(uuid.New().String(), 31)
	klog.Infof("Renaming dest volume %s -> %s before snapshot copy", destName, tempName)
	if err := p.renameVolume(destName, tempName); err != nil {
		return fmt.Errorf("failed to rename dest volume aside: %w", err)
	}

	// Step 1: Create a read-only snapshot of the source with the dest name.
	// This is instant (CoW) — no data is copied.
	klog.Infof("Creating snapshot of %s as %s", sourceName, destName)
	if err := p.createSnapshot(sourceName, destName); err != nil {
		p.rollbackRename(tempName, destName)
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	if progress != nil {
		progress <- 50
	}

	// Step 2: Promote the snapshot to an independent writable volume.
	// With online=true the volume is writable immediately; background
	// data separation happens asynchronously.
	klog.Infof("Promoting snapshot %s to independent volume", destName)
	if err := p.promoteVirtualCopy(destName); err != nil {
		p.deleteVolume(destName)
		p.rollbackRename(tempName, destName)
		return fmt.Errorf("failed to promote snapshot: %w", err)
	}

	if err := p.deleteVolume(tempName); err != nil {
		klog.Warningf("Failed to delete old volume %s (non-fatal): %v", tempName, err)
	}

	return nil
}

func (p *Primera3ParClientWsImpl) rollbackRename(tempName, origName string) {
	klog.Warningf("Rolling back: renaming %s -> %s", tempName, origName)
	if err := p.renameVolume(tempName, origName); err != nil {
		klog.Errorf("Rollback rename failed: %v", err)
	}
}

func (p *Primera3ParClientWsImpl) createSnapshot(sourceVolName, snapshotName string) error {
	reqURL := fmt.Sprintf("%s/api/v1/volumes/%s", p.BaseURL, url.PathEscape(sourceVolName))

	requestBody := map[string]any{
		"action": "createSnapshot",
		"parameters": map[string]any{
			"name": snapshotName,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to encode createSnapshot request: %w", err)
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create createSnapshot request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.doRequest(req, "createSnapshot")
	if err != nil {
		return fmt.Errorf("createSnapshot failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("createSnapshot failed with status %d: %s", resp.StatusCode, string(body))
	}

	klog.Infof("Created snapshot %s of volume %s", snapshotName, sourceVolName)
	return nil
}

const promoteVirtualCopyAction = 4

func (p *Primera3ParClientWsImpl) promoteVirtualCopy(snapshotName string) error {
	reqURL := fmt.Sprintf("%s/api/v1/volumes/%s", p.BaseURL, url.PathEscape(snapshotName))

	requestBody := map[string]any{
		"action": promoteVirtualCopyAction,
		"online": true,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to encode promoteVirtualCopy request: %w", err)
	}

	req, err := http.NewRequest("PUT", reqURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create promoteVirtualCopy request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.doRequest(req, "promoteVirtualCopy")
	if err != nil {
		return fmt.Errorf("promoteVirtualCopy failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read promoteVirtualCopy response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("promoteVirtualCopy failed with status %d: %s", resp.StatusCode, string(body))
	}

	var taskResp struct {
		TaskID int `json:"taskid"`
	}
	if err := json.Unmarshal(body, &taskResp); err == nil && taskResp.TaskID != 0 {
		klog.Infof("promoteVirtualCopy started async task %d, verifying it is running", taskResp.TaskID)
		if err := p.verifyTaskRunning(taskResp.TaskID); err != nil {
			return fmt.Errorf("promote task %d failed verification: %w", taskResp.TaskID, err)
		}
	}

	klog.Infof("Promoted snapshot %s to independent volume", snapshotName)
	return nil
}

func (p *Primera3ParClientWsImpl) verifyTaskRunning(taskID int) error {
	const (
		taskStatusDone   = 1
		taskStatusActive = 2
	)

	taskURL := fmt.Sprintf("%s/api/v1/tasks/%d", p.BaseURL, taskID)
	req, err := http.NewRequest("GET", taskURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create task status request: %w", err)
	}

	var t struct {
		ID     int    `json:"id"`
		Status int    `json:"status"`
		Name   string `json:"name"`
	}
	if err := p.doRequestUnmarshalResponse(req, "verifyTaskRunning", &t); err != nil {
		return fmt.Errorf("failed to get task %d status: %w", taskID, err)
	}

	klog.Infof("Task %d (%s) status: %d", taskID, t.Name, t.Status)
	if t.Status != taskStatusDone && t.Status != taskStatusActive {
		return fmt.Errorf("task %d has unexpected status %d (name: %s)", taskID, t.Status, t.Name)
	}
	return nil
}

func (p *Primera3ParClientWsImpl) GetSystemInfo() (SystemInfo, error) {
	reqURL := fmt.Sprintf("%s/api/v1/system", p.BaseURL)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return SystemInfo{}, fmt.Errorf("failed to create GetSystemInfo request: %w", err)
	}

	var sysInfo SystemInfo
	if err := p.doRequestUnmarshalResponse(req, "GetSystemInfo", &sysInfo); err != nil {
		return SystemInfo{}, fmt.Errorf("failed to get system info: %w", err)
	}

	return sysInfo, nil
}

// wsApiBuild2023 is the WSAPI build number starting from which snapCPG is
// managed at the pool level and must not be set on individual volumes.
const wsApiBuild2023 = 100000000

func (p *Primera3ParClientWsImpl) getWsApiBuild() (int, error) {
	if p.wsApiBuild > 0 {
		return p.wsApiBuild, nil
	}

	reqURL := fmt.Sprintf("%s/api", p.BaseURL)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create WSAPI version request: %w", err)
	}

	var resp struct {
		Build int `json:"build"`
	}
	if err := p.doRequestUnmarshalResponse(req, "getWsApiBuild", &resp); err != nil {
		return 0, fmt.Errorf("failed to get WSAPI version: %w", err)
	}

	p.wsApiBuild = resp.Build
	klog.Infof("WSAPI build version: %d", p.wsApiBuild)
	return p.wsApiBuild, nil
}

func (p *Primera3ParClientWsImpl) GetVolumes(query string) ([]Volume, error) {
	u := fmt.Sprintf("%s/api/v1/volumes", p.BaseURL)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if query != "" {
		encoded := url.PathEscape(fmt.Sprintf("%q", query))
		req.URL.RawQuery = fmt.Sprintf("query=%s", encoded)
	}

	type GetVolumesResponse struct {
		Members []Volume `json:"members"`
	}

	var response GetVolumesResponse

	err = p.doRequestUnmarshalResponse(req, "getVolumes", &response)
	if err != nil {
		return nil, err
	}

	return response.Members, nil
}
