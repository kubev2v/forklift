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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/infinidat/infinibox-csi-driver/common"
)

type ExportPathRef struct {
	InnerPath          string        `json:"inner_path,omitempty"`
	PrefWrite          int           `json:"pref_write,omitempty"`
	PrefRead           int           `json:"pref_read,omitempty"`
	MaxRead            int           `json:"max_read,omitempty"`
	PrefReaddir        int           `json:"pref_readdir,omitempty"`
	TransportProtocols string        `json:"transport_protocols,omitempty"`
	FilesystemID       int           `json:"filesystem_id,omitempty"`
	MaxWrite           int           `json:"max_write,omitempty"`
	PrivilegedPort     bool          `json:"privileged_port"`
	ExportPath         string        `json:"export_path,omitempty"`
	Permissions        []Permissions `json:"permissions,omitempty"`
	SnapdirVisible     bool          `json:"snapdir_visible"`
}
type UpdateExportPathRef struct {
	Permissions []Permissions `json:"permissions,omitempty"`
}

type Permissions struct {
	Access       string `json:"access,omitempty"`
	NoRootSquash bool   `json:"no_root_squash,omitempty"`
	Client       string `json:"client,omitempty"`
}

type Export struct {
	InnerPath             string        `json:"inner_path,omitempty"`
	PrefWrite             int           `json:"pref_write,omitempty"`
	BitFileID             bool          `json:"32bit_file_id,omitempty"`
	PrefRead              int           `json:"pref_read,omitempty"`
	MaxRead               int           `json:"max_read,omitempty"`
	Permissions           []Permissions `json:"permissions,omitempty"`
	TenantID              int           `json:"tenant_id,omitempty"`
	CreatedAt             int           `json:"created_at,omitempty"`
	PrefReaddir           int           `json:"pref_readdir,omitempty"`
	Enabled               bool          `json:"enabled,omitempty"`
	UpdatedAt             int           `json:"updated_at,omitempty"`
	MakeAllUsersAnonymous bool          `json:"make_all_users_anonymous,omitempty"`
	SnapdirVisible        bool          `json:"snapdir_visible,omitempty"`
	TransportProtocols    string        `json:"transport_protocols,omitempty"`
	AnonymousGID          int           `json:"anonymous_gid,omitempty"`
	AnonymousUID          int           `json:"anonymous_uid,omitempty"`
	FilesystemID          int           `json:"filesystem_id,omitempty"`
	MaxWrite              int           `json:"max_write,omitempty"`
	PrivilegedPort        bool          `json:"privileged_port,omitempty"`
	ID                    int           `json:"id,omitempty"`
	ExportPath            string        `json:"export_path,omitempty"`
}

type GetExportByIDResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Export   `json:"result"`
	Error    Error    `json:"error"`
}
type UpdateExportResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Export   `json:"result"`
	Error    Error    `json:"error"`
}

type DeleteExportResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Export   `json:"result"`
	Error    Error    `json:"error"`
}

type GetExportsByFileSystemIDResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   []Export `json:"result"`
	Error    Error    `json:"error"`
}

type CreateExportRequest struct {
	FilesystemID       int                      `json:"filesystem_id,omitempty"`
	Name               string                   `json:"name,omitempty"`
	TransportProtocols string                   `json:"transport_protocols,omitempty"`
	PrivilegedPort     bool                     `json:"privileged_port"`
	ExportPath         string                   `json:"export_path,omitempty"`
	Permissionsput     []map[string]interface{} `json:"permissions,omitempty"`
	SnapdirVisible     bool                     `json:"snapdir_visible"`
}
type CreateExportResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Export   `json:"result"`
	Error    Error    `json:"error"`
}

func (iboxClient *IboxClient) GetExportByID(exportID int) (ex *Export, err error) {
	const functionName = "GetExportByID"

	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/exports", exportID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "export ID", exportID)

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
	var responseObject GetExportByIDResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "EXPORT_NOT_FOUND" {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - export ID '%d' not found", functionName, exportID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) GetExportsByFileSystemID(fsID int) (results []Export, err error) {
	const functionName = "GetExportsByFileSystemID"

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/exports")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "filesystem ID", fsID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return results, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
		}

		values := req.URL.Query()
		values.Add("filesystem_id", strconv.Itoa(fsID))
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
		var responseObject GetExportsByFileSystemIDResponse
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

func (iboxClient *IboxClient) DeleteExport(exportID int) (response *Export, err error) {
	const functionName = "DeleteExport"

	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/exports", exportID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "export ID", exportID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := req.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
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
		return nil, fmt.Errorf("%s - ReadAll -error %w", functionName, err)
	}
	var responseObject DeleteExportResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "EXPORT_NOT_FOUND" {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - export ID '%d' not found", functionName, exportID)}
		}

		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) CreateExport(req CreateExportRequest) (*Export, error) {
	const functionName = "CreateExport"

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/exports")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "request", req)

	jsonBytes, err := json.Marshal(req)
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

	body, _ := io.ReadAll(response.Body)

	var responseObject CreateExportResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "Export ID", responseObject.Result.ID)
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) UpdateExportPermissions(ex Export, exportPathRef ExportPathRef) (resp *Export, err error) {
	const functionName = "UpdateExport"
	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/exports", ex.ID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "export ID", ex.ID, "exportPathRef", exportPathRef)

	// the ibox only allows a single field of the export rule to be updated, in this
	// case we want to only update the Permissions of an existing export rule, this is
	// needed when a Pod moves from one node to another node, requiring the new node's ip address to be
	// covered by an export permission
	onlyPermissionsField := UpdateExportPathRef{
		Permissions: exportPathRef.Permissions,
	}
	jsonBytes, err := json.Marshal(onlyPermissionsField)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	iboxClient.Log.V(DEBUG_LEVEL).Info(functionName, "URL", url, "update export json", string(jsonBytes))
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

	var responseObject UpdateExportResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}
