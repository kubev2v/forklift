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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

func (client *IboxClient) GetExportByID(ctx context.Context, exportID int) (ex *Export, err error) {

	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/exports", exportID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "export ID", exportID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}
	SetAuthHeader(req, client.Creds)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, common.Errorf("do - error %w url: %s", err, url)
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
	var responseObject GetExportByIDResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "EXPORT_NOT_FOUND" {
			return nil, common.Errorf("errorCode: %s - error: %w url: %s", responseObject.Error.Code, ErrNotFound, url)
		}
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return &responseObject.Result, nil
}

func (client *IboxClient) GetExportsByFileSystemID(ctx context.Context, fsID int) (results []Export, err error) {

	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/exports")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "filesystem ID", fsID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return results, common.Errorf("newRequest - error: %w url: %s", err, url)
		}

		values := req.URL.Query()
		values.Add("filesystem_id", strconv.Itoa(fsID))
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
		var responseObject GetExportsByFileSystemIDResponse
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

func (client *IboxClient) DeleteExport(ctx context.Context, exportID int) (response *Export, err error) {

	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/exports", exportID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "export ID", exportID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := req.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
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
	var responseObject DeleteExportResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "EXPORT_NOT_FOUND" {
			return nil, common.Errorf("errorCode: %s - error: %w url: %s", responseObject.Error.Code, ErrNotFound, url)
		}

		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return &responseObject.Result, nil
}

func (client *IboxClient) CreateExport(ctx context.Context, req CreateExportRequest) (*Export, error) {

	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/exports")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "request", req)

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		return nil, common.Errorf("marshal - error: %w url: %s", err, url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}
	SetAuthHeader(request, client.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject CreateExportResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	slog.Log(ctx, common.LevelTrace, "info", "Export ID", responseObject.Result.ID)
	return &responseObject.Result, nil
}

func (client *IboxClient) UpdateExportPermissions(ctx context.Context, ex Export, exportPath ExportPathRef) (resp *Export, err error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/exports", ex.ID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "export ID", ex.ID, "exportPathRef", exportPath)

	// the ibox only allows a single field of the export rule to be updated, in this
	// case we want to only update the Permissions of an existing export rule, this is
	// needed when a Pod moves from one node to another node, requiring the new node's ip address to be
	// covered by an export permission
	onlyPermissionsField := UpdateExportPathRef{
		Permissions: exportPath.Permissions,
	}
	jsonBytes, err := json.Marshal(onlyPermissionsField)
	if err != nil {
		return nil, common.Errorf("marshal - error: %w url: %s", err, url)
	}
	slog.Debug("URL", url, "info", "update export json", string(jsonBytes))
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := request.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	request.URL.RawQuery = values.Encode()

	SetAuthHeader(request, client.Creds)

	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}

	var responseObject UpdateExportResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return &responseObject.Result, nil
}
