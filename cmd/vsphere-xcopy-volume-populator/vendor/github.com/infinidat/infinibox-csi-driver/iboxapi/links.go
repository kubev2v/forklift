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

func (client *IboxClient) GetLinks(ctx context.Context) (results []Link, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/links")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return results, common.Errorf("newRequest - error: %w url: %s", err, url)
		}

		values := req.URL.Query()
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()

		SetAuthHeader(req, client.Creds)

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return results, common.Errorf("do - error: %w url: %s", err, url)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Error("error in Close()", "error", err.Error())
			}
		}()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return results, common.Errorf("readAll - error: %w url: %s", err, url)
		}
		var responseObject GetLinksResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return results, common.Errorf("unmarshal - error: %w url: %s", err, url)
		}
		results = append(results, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return results, nil
}

func (client *IboxClient) GetLink(ctx context.Context, linkID int) (link *Link, err error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/links", linkID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "link ID", linkID)

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
	var responseObject GetLinkResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "LINK_NOT_FOUND" {
			return nil, common.Errorf("errorCode: %s - error: %w url: %s", responseObject.Error.Code, ErrNotFound, url)
		}
		return nil, common.Errorf("ibox API - errorcode: %s message: %s, url: %s", responseObject.Error.Code, responseObject.Error.Message, url)
	}
	return &responseObject.Result, nil
}
