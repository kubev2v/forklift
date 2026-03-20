package iboxapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

func (iboxClient *IboxClient) GetAllHosts() (host []Host, err error) {
	const functionName = "GetAllHosts"
	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/hosts")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return host, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}
	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return host, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return host, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}
	var responseObject HostResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return host, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return host, fmt.Errorf("%s - ibox API - error code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}

	return responseObject.Result, nil
}

func (iboxClient *IboxClient) GetHostByName(hostName string) (host *Host, err error) {
	const functionName = "GetHostByName"
	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/hosts")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "host name", hostName)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}
	SetAuthHeader(req, iboxClient.Creds)

	values := req.URL.Query()
	values.Add("name", hostName)
	req.URL.RawQuery = values.Encode()

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}
	var responseObject HostResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if len(responseObject.Result) == 0 {
		return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - host '%s' not found", functionName, hostName)}
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result[0], nil
}

func (iboxClient *IboxClient) CreateHost(hostName string) (*Host, error) {
	const functionName = "CreateHost"
	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/hosts")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "host name", hostName)

	hostPort := CreateHostPost{
		Name: hostName,
	}
	jsonBytes, err := json.Marshal(hostPort)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}
	SetAuthHeader(request, iboxClient.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := iboxClient.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%s -ReadAll - error %w", functionName, err)
	}

	var responseObject CreateHostResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) DeleteHost(hostID int) (response *Host, err error) {
	const functionName = "DeleteHost"
	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/hosts/", hostID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "host ID", hostID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRquest -  error %w", functionName, err)
	}
	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}

	var responseObject DeleteHostResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "HOST_NOT_FOUND" {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - host ID '%d' not found", functionName, hostID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}

	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) AddHostSecurity(chapCreds map[string]string, hostID int) (host *AddHostSecurityResponse, err error) {
	const functionName = "AddHostSecurity"
	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/hosts/", hostID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "host ID", hostID)

	hostSecurityInfo := AddHostSecurityRequest{
		SecurityMethod:               chapCreds[CHAP_SECURITY_METHOD],
		SecurityCHAPInboundUsername:  chapCreds[CHAP_INBOUND_USERNAME],
		SecurityCHAPInboundSecret:    chapCreds[CHAP_INBOUND_SECRET],
		SecurityCHAPOutboundUsername: chapCreds[CHAP_OUTBOUND_USERNAME],
		SecurityCHAPOutboundSecret:   chapCreds[CHAP_OUTBOUND_SECRET],
	}

	jsonBytes, err := json.Marshal(hostSecurityInfo)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	request, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := request.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	request.URL.RawQuery = values.Encode()

	SetAuthHeader(request, iboxClient.Creds)

	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := iboxClient.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}

	var responseObject AddHostSecurityResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject, nil
}

func (iboxClient *IboxClient) AddHostPort(portType, portAddress string, hostID int) (addPortResponse *AddPortResponse, err error) {
	const functionName = "AddHostPort"
	url := fmt.Sprintf("%s%s/%d/ports", iboxClient.Creds.URL, "api/rest/hosts", hostID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "port type", portType, "port address", portAddress, "host ID", hostID)

	portInfo := AddPortRequest{
		Type:    portType,
		Address: portAddress,
	}

	jsonBytes, err := json.Marshal(portInfo)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := request.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	request.URL.RawQuery = values.Encode()

	SetAuthHeader(request, iboxClient.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := iboxClient.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject AddPortResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal -error %w", functionName, err)
	}
	return &responseObject, nil
}

func (iboxClient *IboxClient) GetHostPort(hostID int, portAddress string) (hostPort *HostPort, err error) {
	const functionName = "GetHostPort"
	url := fmt.Sprintf("%s%s/%d/ports", iboxClient.Creds.URL, "api/rest/hosts", hostID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "host ID", hostID, "port address", portAddress)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}
	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}
	var responseObject GetHostPortResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	var portFound bool
	for _, port := range responseObject.Result {
		if port.PortAddress == portAddress {
			hostPort = &port
			portFound = true
		}
	}
	if !portFound {
		return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - portAddress '%s' not found", functionName, portAddress)}
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return hostPort, nil
}

func (iboxClient *IboxClient) MapVolumeToHost(hostID, volumeID, lun int) (lunInfo *LunInfo, err error) {
	const functionName = "MapVolumeToHost"
	url := fmt.Sprintf("%s%s/%d/luns", iboxClient.Creds.URL, "api/rest/hosts", hostID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "volume ID", volumeID, "lun", lun, "host ID", hostID)

	hp := MapVolumeToHostRequest{
		VolumeID: volumeID,
	}

	jsonBytes, err := json.Marshal(hp)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := request.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	request.URL.RawQuery = values.Encode()

	SetAuthHeader(request, iboxClient.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := iboxClient.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject MapVolumeToHostResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) GetAllLunByHost(hostID int) (luns []LunInfo, err error) {
	const functionName = "GetAllLunByHost"
	url := fmt.Sprintf("%s%s/%d/luns", iboxClient.Creds.URL, "api/rest/hosts/", hostID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "host ID", hostID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.

	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return luns, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
		}
		values := req.URL.Query()
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages, "URL", req.URL.RawQuery)

		SetAuthHeader(req, iboxClient.Creds)

		resp, err := iboxClient.HTTPClient.Do(req)
		if err != nil {
			return luns, fmt.Errorf("%s - Do - error %w", functionName, err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return luns, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
		}
		var responseObject GetAllLunsResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return luns, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
		}

		luns = append(luns, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return luns, nil
}

func (iboxClient *IboxClient) GetLunByHostVolume(hostID, volumeID int) (lun *LunInfo, err error) {
	const functionName = "GetLunByHostVolume"
	url := fmt.Sprintf("%s%s/%d/luns", iboxClient.Creds.URL, "api/rest/hosts/", hostID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "host ID", hostID, "volume ID", volumeID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.

	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
		}
		values := req.URL.Query()
		values.Add("volume_id", strconv.Itoa(volumeID))
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages, "URL", req.URL.RawQuery)

		SetAuthHeader(req, iboxClient.Creds)

		resp, err := iboxClient.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
		}
		var responseObject GetAllLunsResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
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
		return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - host ID '%d' volume ID '%d' not found", functionName, hostID, volumeID)}
	}

	return lun, nil
}

func (iboxClient *IboxClient) UnMapVolumeFromHost(hostID, volumeID int) (unmapResponse *UnMapVolumeFromHostResponse, err error) {
	const functionName = "UnMapVolumeFromHost"
	url := fmt.Sprintf("%s%s/%d/luns/volume_id/%d", iboxClient.Creds.URL, "api/rest/hosts/", hostID, volumeID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "host ID", hostID, "volume ID", volumeID)

	request, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := request.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	request.URL.RawQuery = values.Encode()

	SetAuthHeader(request, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}

	var responseObject UnMapVolumeFromHostResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject, nil
}
