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

type GetLinksResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   []Link   `json:"result"`
	Error    Error    `json:"error"`
}
type GetLinkResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Link     `json:"result"`
	Error    Error    `json:"error"`
}

type Link struct {
	ID                              int    `json:"id"`
	Version                         int    `json:"_version"`
	RemoteVersion                   string `json:"remote_version"`
	Name                            string `json:"name"`
	RemoteHost                      string `json:"remote_host"`
	RemoteManagementIP              string `json:"_remote_management_ip"`
	RemoteLinkID                    int    `json:"remote_link_id"`
	RemoteReplicationNetworkSpaceID int    `json:"remote_replication_network_space_id"`
	RemoteSystemSerialNumber        int    `json:"remote_system_serial_number"`
	RemoteSystemName                string `json:"remote_system_name"`
	ConnectTimeout                  int    `json:"connect_timeout"`
	KeepAliveTime                   int    `json:"keep_alive_time"`
	RetryCount                      int    `json:"retry_count"`
	RetryWait                       int    `json:"retry_wait"`
	RemoteReplicationIPAddresses    []struct {
		ID         int    `json:"id"`
		IPAddress  string `json:"ip_address"`
		Local      bool   `json:"local"`
		Management bool   `json:"management"`
		Type       string `json:"type"`
		LinkID     int    `json:"link_id"`
	} `json:"remote_replication_ip_addresses"`
	LinkState                      string   `json:"link_state"`
	StateDescription               any      `json:"state_description"`
	LastConnectionTimestamp        int64    `json:"last_connection_timestamp"`
	LocalHost                      any      `json:"_local_host"`
	WitnessAddress                 any      `json:"witness_address"`
	LinkConfigurationGUID          string   `json:"_link_configuration_guid"`
	LinkMode                       string   `json:"link_mode"`
	ResiliencyMode                 string   `json:"resiliency_mode"`
	LocalWitnessState              string   `json:"local_witness_state"`
	LocalWitnessStateDescription   string   `json:"local_witness_state_description"`
	RemoteWitnessState             string   `json:"remote_witness_state"`
	LocalReplicationNetworkSpaceID int      `json:"local_replication_network_space_id"`
	AsyncOnly                      bool     `json:"async_only"`
	IsLocalLinkReadyForSync        bool     `json:"is_local_link_ready_for_sync"`
	LocalLinkReplicationType       []string `json:"local_link_replication_type"`
	LinkReplicationType            []string `json:"link_replication_type"`
}

func (iboxClient *IboxClient) GetLinks() (results []Link, err error) {
	const functionName = "GetLinks"
	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/links")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return results, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
		}

		values := req.URL.Query()
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()

		SetAuthHeader(req, iboxClient.Creds)

		resp, err := iboxClient.HTTPClient.Do(req)
		if err != nil {
			return results, fmt.Errorf("%s - Do - error %w", functionName, err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
			}
		}()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return results, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
		}
		var responseObject GetLinksResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return results, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
		}
		results = append(results, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return results, nil
}

func (iboxClient *IboxClient) GetLink(linkID int) (link *Link, err error) {
	const functionName = "GetLink"
	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/links", linkID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "link ID", linkID)

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
	var responseObject GetLinkResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		// TODO  check for NOT FOUND?  return ErrNotFound for callers?
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}
