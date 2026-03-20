package iboxapi

/*
Copyright 2025 Infinidat
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/infinidat/infinibox-csi-driver/common"
)

type Portal struct {
	Type        string `json:"type,omitempty"`
	Tpgt        int    `json:"tpgt,omitempty"`
	IPAddress   string `json:"ip_address,omitempty"`
	VlanID      int    `json:"vlan_id,omitempty"`
	Enabled     bool   `json:"enabled,omitempty"`
	Reserved    bool   `json:"reserved,omitempty"`
	InterfaceID int    `json:"interface_id,omitempty"`
}

type NetworkSpaceProperty struct {
	ISCSIServer         interface{} `json:"iscsi_isns_servers,omitempty"`
	ISCSIIqn            string      `json:"iscsi_iqn,omitempty"`
	ISCSITCPPort        int         `json:"iscsi_tcp_port,omitempty"`
	ISCSISecurityMethod string      `json:"iscsi_default_security_method,omitempty"`
}
type NetworkConfigDetails struct {
	Netmask        int    `json:"netmask,omitempty"`
	Metwork        string `json:"network,omitempty"`
	DefaultGateway string `json:"default_gateway,omitempty"`
}
type VmacAddress struct {
	Role        string `json:"role,omitempty"`
	VmacAddress string `json:"vmac_address,omitempty"`
}
type Route struct {
	Netmask     int    `json:"netmask,omitempty"`
	Destination string `json:"destination,omitempty"`
	ID          int    `json:"id,omitempty"`
	Gateway     string `json:"gateway,omitempty"`
}

type NetworkSpace struct {
	Properties          NetworkSpaceProperty `json:"properties,omitempty"`
	Service             string               `json:"service,omitempty"`
	TenantID            int                  `json:"tenant_id,omitempty"`
	AutomaticIPFailback bool                 `json:"automatic_ip_failback,omitempty"`
	Interfaces          interface{}          `json:"interfaces,omitempty"`
	RateLimit           interface{}          `json:"rate_limit,omitempty"`
	ID                  int                  `json:"id,omitempty"`
	Portals             []Portal             `json:"ips,omitempty"`
	Mtu                 int                  `json:"mtu,omitempty"`
	NetworkConfig       NetworkConfigDetails `json:"network_config,omitempty"`
	Name                string               `json:"name,omitempty"`
	VmacAddresses       []VmacAddress        `json:"vmac_addresses,omitempty"`
	Routes              []Route              `json:"routes,omitempty"`
}

type GetNetworkSpaceByNameResponse struct {
	Metadata Metadata       `json:"metadata"`
	Result   []NetworkSpace `json:"result"`
	Error    Error          `json:"error"`
}

func (iboxClient *IboxClient) GetNetworkSpaceByName(netspaceName string) (networkSpace *NetworkSpace, err error) {
	const functionName = "GetNetworkSpaceByName"
	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/network/spaces")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "net space Name", netspaceName)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
		}

		values := req.URL.Query()
		values.Add("name", netspaceName)
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()

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
		var responseObject GetNetworkSpaceByNameResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
		}
		if responseObject.Error.Code != "" {
			// TODO check for NOT FOUND?  return ErrNotFound for callers?
			return nil, fmt.Errorf("%s - ibox API - error code %s message %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
		}
		if len(responseObject.Result) > 0 {
			networkSpace = &responseObject.Result[0]
		} else {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - netspace name '%s' not found", functionName, netspaceName)}
		}

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return networkSpace, nil
}
