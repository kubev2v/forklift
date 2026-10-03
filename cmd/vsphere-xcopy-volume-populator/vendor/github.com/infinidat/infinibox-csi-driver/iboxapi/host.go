package iboxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/infinidat/infinibox-csi-driver/common"
)

const (
	CHAP_SECURITY_METHOD   = "security_method"
	CHAP_INBOUND_USERNAME  = "security_chap_inbound_username"
	CHAP_INBOUND_SECRET    = "security_chap_inbound_secret"
	CHAP_OUTBOUND_USERNAME = "security_chap_outbound_username"
	CHAP_OUTBOUND_SECRET   = "security_chap_outbound_secret"
)

type AddHostSecurityRequest struct {
	SecurityMethod               string `json:"security_method"`
	SecurityCHAPInboundUsername  string `json:"security_chap_inbound_username,omitempty"`
	SecurityCHAPInboundSecret    string `json:"security_chap_inbound_secret,omitempty"`
	SecurityCHAPOutboundUsername string `json:"security_chap_outbound_username,omitempty"`
	SecurityCHAPOutboundSecret   string `json:"security_chap_outbound_secret,omitempty"`
}

type CreateHostPost struct {
	Name string `json:"name"`
}

type CreateHostResponse struct {
	Result   Host               `json:"result"`
	Error    Error              `json:"error"`
	Metadata CreateHostMetadata `json:"metadata"`
}

type CreateHostMetadata struct {
	Ready bool `json:"ready"`
}

type DeleteHostResponse struct {
	Result   Host               `json:"result"`
	Error    Error              `json:"error"`
	Metadata CreateHostMetadata `json:"metadata"`
}

type HostResponse struct {
	Result   []Host   `json:"result"`
	Error    Error    `json:"error"`
	Metadata Metadata `json:"metadata"`
}
type Ports struct {
	Address string `json:"address"`
	Type    string `json:"type"`
	HostID  int    `json:"host_id"`
}
type LunInfo struct {
	ID            int  `json:"id,omitempty"`
	Lun           int  `json:"lun,omitempty"`
	CLustered     bool `json:"clustered,omitempty"`
	VolumeID      int  `json:"volume_id,omitempty"`
	HostClusterID int  `json:"host_cluster_id,omitempty"`
	HostID        int  `json:"host_id,omitempty"`
	Udid          any  `json:"udid,omitempty"`
}

type GetAllLunsResponse struct {
	Result   []LunInfo `json:"result"`
	Error    Error     `json:"error"`
	Metadata Metadata  `json:"metadata"`
}

type UnMapVolumeFromHostResponse struct {
	Result   LunInfo  `json:"result"`
	Error    Error    `json:"error"`
	Metadata Metadata `json:"metadata"`
}

type Host struct {
	ID                            int       `json:"id"`
	Name                          string    `json:"name"`
	Ports                         []Ports   `json:"ports"`
	Luns                          []LunInfo `json:"luns"`
	CreatedAt                     int64     `json:"created_at"`
	UpdatedAt                     int64     `json:"updated_at"`
	HostType                      string    `json:"host_type"`
	SecurityMethod                string    `json:"security_method"`
	SecurityChapInboundUsername   any       `json:"security_chap_inbound_username"`
	SecurityChapOutboundUsername  any       `json:"security_chap_outbound_username"`
	Optimized                     bool      `json:"optimized"`
	SanClientType                 string    `json:"san_client_type"`
	HostClusterID                 int       `json:"host_cluster_id"`
	SubsystemNqn                  any       `json:"subsystem_nqn"`
	SecurityChapHasInboundSecret  bool      `json:"security_chap_has_inbound_secret"`
	SecurityChapHasOutboundSecret bool      `json:"security_chap_has_outbound_secret"`
	TenantID                      int       `json:"tenant_id"`
}

type AddHostSecurityResponse struct {
	Result   Host               `json:"result"`
	Error    Error              `json:"error"`
	Metadata CreateHostMetadata `json:"metadata"`
}

type AddPortRequest struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}

type HostPort struct {
	HostID      int    `json:"host_id,omitempty"`
	PortType    string `json:"type,omitempty"`
	PortAddress string `json:"address,omitempty"`
}

type GetHostPortResponse struct {
	Metadata Metadata   `json:"metadata"`
	Result   []HostPort `json:"result"`
	Error    Error      `json:"error"`
}

type AddPortResponse struct {
	Metadata Metadata      `json:"metadata"`
	Result   AddPortResult `json:"result"`
	Error    Error         `json:"error"`
}
type AddPortResult struct {
	HostID  int    `json:"host_id"`
	Type    string `json:"type"`
	Address string `json:"address"`
}

type MapVolumeToHostRequest struct {
	VolumeID int `json:"volume_id"`
}

type MapVolumeToHostResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   LunInfo  `json:"result"`
	Error    Error    `json:"error"`
}

func (client *IboxClient) GetAllHosts(ctx context.Context) (hosts []Host, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/hosts")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.

	for page := 1; page <= totalPages; page++ {
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return hosts, common.Errorf("newRequest - error: %w url: %s", err, url)
		}
		values := req.URL.Query()
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages, "URL", req.URL.RawQuery)

		SetAuthHeader(req, client.Creds)

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return hosts, common.Errorf("do - error: %w url: %s", err, url)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Error("error in Close()", "error", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return hosts, common.Errorf("readAll - error: %w url: %s", err, url)
		}
		var response HostResponse
		err = json.Unmarshal(bodyBytes, &response)
		if err != nil {
			return hosts, common.Errorf("unmarshal - error: %w url: %s", err, url)
		}
		if response.Error.Code != "" {
			return hosts, common.Errorf("ibox API - error: %v url: %s", response.Error, url)
		}

		hosts = append(hosts, response.Result...)

		if page == 1 {
			totalPages = response.Metadata.PagesTotal
		}
	}

	return hosts, nil
}

func (client *IboxClient) GetHostByName(ctx context.Context, hostName string) (host *Host, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/hosts")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "host name", hostName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}
	SetAuthHeader(req, client.Creds)

	values := req.URL.Query()
	values.Add("name", hostName)
	req.URL.RawQuery = values.Encode()

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}
	var responseObject HostResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if len(responseObject.Result) == 0 {
		return nil, ErrNotFound
	}
	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return &responseObject.Result[0], nil
}

func (client *IboxClient) CreateHost(ctx context.Context, hostName string) (*Host, error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/hosts")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "host name", hostName)

	hostPort := CreateHostPost{
		Name: hostName,
	}
	jsonBytes, err := json.Marshal(hostPort)
	if err != nil {
		return nil, common.Errorf("marshal - error: %w url: %s", err, url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}
	SetAuthHeader(request, client.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}

	var responseObject CreateHostResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return &responseObject.Result, nil
}

func (client *IboxClient) DeleteHost(ctx context.Context, hostID int) (response *Host, err error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/hosts/", hostID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "host ID", hostID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, common.Errorf("newRquest -  error: %w url: %s", err, url)
	}
	SetAuthHeader(req, client.Creds)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}

	var responseObject DeleteHostResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "HOST_NOT_FOUND" {
			return nil, common.Errorf("errorCode: %s - error: %w url: %s", responseObject.Error.Code, ErrNotFound, url)
		}
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}

	return &responseObject.Result, nil
}

func (client *IboxClient) AddHostSecurity(ctx context.Context, chapCreds map[string]string, hostID int) (host *AddHostSecurityResponse, err error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/hosts/", hostID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "host ID", hostID)

	hostSecurityInfo := AddHostSecurityRequest{
		SecurityMethod:               chapCreds[CHAP_SECURITY_METHOD],
		SecurityCHAPInboundUsername:  chapCreds[CHAP_INBOUND_USERNAME],
		SecurityCHAPInboundSecret:    chapCreds[CHAP_INBOUND_SECRET],
		SecurityCHAPOutboundUsername: chapCreds[CHAP_OUTBOUND_USERNAME],
		SecurityCHAPOutboundSecret:   chapCreds[CHAP_OUTBOUND_SECRET],
	}

	jsonBytes, err := json.Marshal(hostSecurityInfo)
	if err != nil {
		return nil, common.Errorf("marshal - error: %w url: %s", err, url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := request.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	request.URL.RawQuery = values.Encode()

	SetAuthHeader(request, client.Creds)

	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}

	var responseObject AddHostSecurityResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return &responseObject, nil
}

func (client *IboxClient) AddHostPort(ctx context.Context, portType, portAddress string, hostID int) (addPortResponse *AddPortResponse, err error) {
	url := fmt.Sprintf("%s%s/%d/ports", client.Creds.URL, "api/rest/hosts", hostID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "port type", portType, "port address", portAddress, "host ID", hostID)

	portInfo := AddPortRequest{
		Type:    portType,
		Address: portAddress,
	}

	jsonBytes, err := json.Marshal(portInfo)
	if err != nil {
		return nil, common.Errorf("marshal - error: %w url: %s", err, url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := request.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	request.URL.RawQuery = values.Encode()

	SetAuthHeader(request, client.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject AddPortResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal -error: %w url: %s", err, url)
	}
	return &responseObject, nil
}

func (client *IboxClient) GetHostPort(ctx context.Context, hostID int, portAddress string) (hostPort *HostPort, err error) {
	url := fmt.Sprintf("%s%s/%d/ports", client.Creds.URL, "api/rest/hosts", hostID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "host ID", hostID, "port address", portAddress)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}
	SetAuthHeader(req, client.Creds)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}
	var response GetHostPortResponse
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	var portFound bool
	for _, port := range response.Result {
		if port.PortAddress == portAddress {
			hostPort = &port
			portFound = true
		}
	}
	if !portFound {
		return nil, ErrNotFound
	}
	if response.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", response.Error, url)
	}
	return hostPort, nil
}

func (client *IboxClient) MapVolumeToHost(ctx context.Context, hostID, volumeID, lun int) (lunInfo *LunInfo, err error) {
	url := fmt.Sprintf("%s%s/%d/luns", client.Creds.URL, "api/rest/hosts", hostID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "volume ID", volumeID, "lun", lun, "host ID", hostID)

	hp := MapVolumeToHostRequest{
		VolumeID: volumeID,
	}

	jsonBytes, err := json.Marshal(hp)
	if err != nil {
		return nil, common.Errorf("marshal - error: %w url: %s", err, url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := request.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	request.URL.RawQuery = values.Encode()

	SetAuthHeader(request, client.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject MapVolumeToHostResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "MAPPING_ALREADY_EXISTS" {
			return nil, ErrMappingExists
		}
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return &responseObject.Result, nil
}

func (client *IboxClient) GetAllLunByHost(ctx context.Context, hostID int) (luns []LunInfo, err error) {
	url := fmt.Sprintf("%s%s/%d/luns", client.Creds.URL, "api/rest/hosts/", hostID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "host ID", hostID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.

	for page := 1; page <= totalPages; page++ {
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return luns, common.Errorf("newRequest - error: %w url: %s", err, url)
		}
		values := req.URL.Query()
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages, "URL", req.URL.RawQuery)

		SetAuthHeader(req, client.Creds)

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return luns, common.Errorf("do - error: %w url: %s", err, url)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Error("error in Close()", "error", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return luns, common.Errorf("readAll - error: %w url: %s", err, url)
		}
		var responseObject GetAllLunsResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return luns, common.Errorf("unmarshal - error: %w url: %s", err, url)
		}

		luns = append(luns, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return luns, nil
}

func (client *IboxClient) GetLunByHostVolume(ctx context.Context, hostID, volumeID int) (lun *LunInfo, err error) {
	url := fmt.Sprintf("%s%s/%d/luns", client.Creds.URL, "api/rest/hosts/", hostID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "host ID", hostID, "volume ID", volumeID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.

	for page := 1; page <= totalPages; page++ {
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
		}
		values := req.URL.Query()
		values.Add("volume_id", strconv.Itoa(volumeID))
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages, "URL", req.URL.RawQuery)

		SetAuthHeader(req, client.Creds)

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return nil, common.Errorf("do - error: %w url: %s", err, url)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Error("error in Close()", "error", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, common.Errorf("readAll - error: %w url: %s", err, url)
		}
		var responseObject GetAllLunsResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
		}

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}

		if len(responseObject.Result) > 0 {
			lun = &responseObject.Result[0]

			break
		}
	}

	if lun == nil {
		return nil, ErrNotFound
	}

	return lun, nil
}

func (client *IboxClient) UnMapVolumeFromHost(ctx context.Context, hostID, volumeID int) (response *UnMapVolumeFromHostResponse, err error) {
	url := fmt.Sprintf("%s%s/%d/luns/volume_id/%d", client.Creds.URL, "api/rest/hosts/", hostID, volumeID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "host ID", hostID, "volume ID", volumeID)

	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := request.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	request.URL.RawQuery = values.Encode()

	SetAuthHeader(request, client.Creds)

	resp, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}

	var responseObject UnMapVolumeFromHostResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return &responseObject, nil
}
