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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := req.URL.Query()
	values.Add("fields", "nas.max_filesystems_in_system")
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, client.Creds)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return 0, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, common.Errorf("readAll - error: %w url: %s", err, url)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := req.URL.Query()
	values.Add("fields", "nas.treeq_max_count_per_filesystem")
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, client.Creds)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return 0, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("Close()", "error", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, common.Errorf("readAll - error: %w url: %s", err, url)
	}
	var responseObject ParameterResult
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return 0, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	return responseObject.Result.NasTreeqMaxCountPerFilesystem, nil
}
