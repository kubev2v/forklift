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
)

func (iboxClient *IboxClient) GetMaxFileSystems() (cnt int, err error) {
	const functionName = "GetMaxFileSystems"

	type ParameterResult struct {
		Result struct {
			NasMaxFilesystemsInSystem int `json:"nas.max_filesystems_in_system"`
		} `json:"result"`
		Error    interface{} `json:"error"`
		Metadata struct {
			Ready bool `json:"ready"`
		} `json:"metadata"`
	}

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/config/limits")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := req.URL.Query()
	values.Add("fields", "nas.max_filesystems_in_system")
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}
	var responseObject ParameterResult
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return 0, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	return responseObject.Result.NasMaxFilesystemsInSystem, nil
}

func (iboxClient *IboxClient) GetMaxTreeqPerFs() (cnt int, err error) {
	const functionName = "GetMaxTreeqPerFs"

	type ParameterResult struct {
		Result struct {
			NasTreeqMaxCountPerFilesystem int `json:"nas.treeq_max_count_per_filesystem"`
		} `json:"result"`
		Error    interface{} `json:"error"`
		Metadata struct {
			Ready bool `json:"ready"`
		} `json:"metadata"`
	}

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/config/limits")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := req.URL.Query()
	values.Add("fields", "nas.treeq_max_count_per_filesystem")
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}
	var responseObject ParameterResult
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return 0, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	return responseObject.Result.NasTreeqMaxCountPerFilesystem, nil
}
