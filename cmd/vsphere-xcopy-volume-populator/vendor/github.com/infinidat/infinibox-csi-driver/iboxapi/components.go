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

	"github.com/infinidat/infinibox-csi-driver/common"
)

type FCPort struct {
	PortID       int    `json:"id"`
	State        string `json:"state,omitempty"`
	Enabled      bool   `json:"enabled,omitempty"`
	WWNn         string `json:"wwnn,omitempty"`
	WWPn         string `json:"wwpn,omitempty"`
	SwitchWWNn   string `json:"switch_wwnn,omitempty"`
	Vendor       string `json:"vendor,omitempty"`
	SwitchVendor string `json:"switch_vendor,omitempty"`
}

type FCNode struct {
	Ports []FCPort `json:"fc_ports,omitempty"`
}

type GetFCPortsResponse struct {
	Result   []FCNode `json:"result"`
	Error    Error    `json:"error"`
	Metadata Metadata `json:"metadata"`
}

func (client *IboxClient) GetFCPorts(ctx context.Context) (nodes []FCNode, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/components/nodes")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nodes, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := req.URL.Query()
	values.Add("fields", "fc_ports")
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, client.Creds)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nodes, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nodes, common.Errorf("readAll - error: %w url: %s", err, url)
	}
	var responseObject GetFCPortsResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nodes, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return nodes, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}

	return responseObject.Result, nil
}
