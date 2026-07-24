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
	"log/slog"

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

	parameters := make(map[string]string)
	parameters["fields"] = "fc_ports"

	bodyBytes, err := commonGetLogic(ctx, url, client, parameters)
	if err != nil {
		return nodes, common.Errorf("commonGetLogic - error: %w url: %s", err, url)
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
