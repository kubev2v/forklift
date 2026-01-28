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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

func (client *IboxClient) GetNetworkSpaceByName(ctx context.Context, netspaceName string) (networkSpace *NetworkSpace, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/network/spaces")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "net space Name", netspaceName)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
		}

		values := req.URL.Query()
		values.Add("name", netspaceName)
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()

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
		var response GetNetworkSpaceByNameResponse
		err = json.Unmarshal(bodyBytes, &response)
		if err != nil {
			return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
		}
		if response.Error.Code != "" {
			return nil, common.Errorf("ibox API - error: %v url: %s", response.Error, url)
		}
		if len(response.Result) == 0 {
			return nil, ErrNotFound
		}
		networkSpace = &response.Result[0]

		if page == 1 {
			totalPages = response.Metadata.PagesTotal
		}
	}

	return networkSpace, nil
}
