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

func (client *IboxClient) GetMaxFileSystems(ctx context.Context) (cnt int, err error) {
	type ParameterResult struct {
		Result struct {
			NasMaxFilesystemsInSystem int `json:"nas.max_filesystems_in_system"`
		} `json:"result"`
		Error    interface{} `json:"error"`
		Metadata struct {
			Ready bool `json:"ready"`
		} `json:"metadata"`
	}

	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/config/limits")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url)

	parameters := make(map[string]string)
	parameters["fields"] = "nas.max_filesystems_in_system"

	bodyBytes, err := commonGetLogic(ctx, url, client, parameters)
	if err != nil {
		return 0, common.Errorf("commonGetLogic - error: %w url: %s", err, url)
	}
	var responseObject ParameterResult
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return 0, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	return responseObject.Result.NasMaxFilesystemsInSystem, nil
}

func (client *IboxClient) GetMaxTreeqPerFs(ctx context.Context) (cnt int, err error) {
	type ParameterResult struct {
		Result struct {
			NasTreeqMaxCountPerFilesystem int `json:"nas.treeq_max_count_per_filesystem"`
		} `json:"result"`
		Error    any `json:"error"`
		Metadata struct {
			Ready bool `json:"ready"`
		} `json:"metadata"`
	}

	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/config/limits")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url)

	parameters := make(map[string]string)
	parameters["fields"] = "nas.treeq_max_count_per_filesystem"

	bodyBytes, err := commonGetLogic(ctx, url, client, parameters)
	if err != nil {
		return 0, common.Errorf("commonGetLogic - error: %w url: %s", err, url)
	}
	var responseObject ParameterResult
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return 0, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	return responseObject.Result.NasTreeqMaxCountPerFilesystem, nil
}
