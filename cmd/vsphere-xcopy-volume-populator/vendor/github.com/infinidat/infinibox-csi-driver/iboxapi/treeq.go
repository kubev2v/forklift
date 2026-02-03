package iboxapi

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

type Treeq struct {
	ID           int    `json:"id,omitempty"`
	FilesystemID int    `json:"filesystem_id,omitempty"`
	Name         string `json:"name,omitempty"`
	Path         string `json:"path,omitempty"`
	HardCapacity int64  `json:"hard_capacity,omitempty"`
	UsedCapacity int64  `json:"used_capacity,omitempty"`
}

type GetTreeqByNameResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   []Treeq  `json:"result"`
	Error    Error    `json:"error"`
}
type GetTreeqByFileSystemResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   []Treeq  `json:"result"`
	Error    Error    `json:"error"`
}
type GetFileSystemTreeqCountResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   []Treeq  `json:"result"`
	Error    Error    `json:"error"`
}

type GetTreeqResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Treeq    `json:"result"`
	Error    Error    `json:"error"`
}

type CreateTreeqRequest struct {
	SoftInodes   int    `json:"soft_inodes,omitempty"`
	Path         string `json:"path"`
	HardCapacity int64  `json:"hard_capacity"`
	HardInodes   int    `json:"hard_inodes,omitempty"`
	Name         string `json:"name"`
}
type CreateTreeqResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Treeq    `json:"result"`
	Error    Error    `json:"error"`
}

type DeleteTreeqResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Treeq    `json:"result"`
	Error    Error    `json:"error"`
}
type UpdateTreeqRequest struct {
	SoftInodes   int    `json:"soft_inodes,omitempty"`
	Path         string `json:"path,omitempty"`
	HardCapacity int64  `json:"hard_capacity,omitempty"`
	HardInodes   int    `json:"hard_inodes,omitempty"`
	Name         string `json:"name,omitempty"`
}
type UpdateTreeqResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Treeq    `json:"result"`
	Error    Error    `json:"error"`
}

const TREEQ_ID_DOES_NOT_EXIST = "TREEQ_ID_DOES_NOT_EXIST"

func (client *IboxClient) GetTreeqByName(ctx context.Context, fsID int, name string) (treeq *Treeq, err error) {
	url := fmt.Sprintf("%s%s/%d/treeqs", client.Creds.URL, "api/rest/filesystems", fsID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "filesystem ID", fsID, "treeq name", name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := req.URL.Query()
	values.Add("name", name)
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
	var response GetTreeqByNameResponse
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	if response.Error.Code != "" {
		if response.Error.Code == FILESYSTEM_NOT_FOUND {
			return nil, common.Errorf("errorCode: %s - error: %w url: %s", response.Error.Code, ErrNotFound, url)
		}
		return nil, common.Errorf("ibox API - error: %v url: %s", response.Error, url)
	}

	if len(response.Result) == 0 {
		return nil, ErrNotFound
	}
	return &response.Result[0], nil
}

func (client *IboxClient) GetTreeq(ctx context.Context, fsID, treeqID int) (treeq *Treeq, err error) {
	url := fmt.Sprintf("%s%s/%d/treeqs/%d", client.Creds.URL, "api/rest/filesystems", fsID, treeqID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "fs ID", fsID, "treeq ID", treeqID)

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
	var response GetTreeqResponse
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	if response.Error.Code != "" {
		if response.Error.Code == TREEQ_ID_DOES_NOT_EXIST {
			return nil, common.Errorf("errorCode: %s - error: %w url; %s", response.Error.Code, ErrNotFound, url)
		}
		return nil, common.Errorf("ibox API - error: %v url: %s", response.Error, url)
	}
	return &response.Result, nil
}

func (client *IboxClient) DeleteTreeq(ctx context.Context, fsID, treeqID int) (treeq *Treeq, err error) {
	url := fmt.Sprintf("%s%s/%d/treeq/%d", client.Creds.URL, "api/rest/filesystems", fsID, treeqID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "fs ID", fsID, "treeq ID", treeqID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, common.Errorf("newRequest -  error: %w url: %s", err, url)
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

	var response DeleteTreeqResponse
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	if response.Error.Code != "" {
		if response.Error.Code == TREEQ_ID_DOES_NOT_EXIST {
			return nil, common.Errorf("errorCode: %s - error: %w url: %s", response.Error.Code, ErrNotFound, url)
		}
		return nil, common.Errorf("ibox API - error: %v url: %s", response.Error, url)
	}

	return &response.Result, nil
}

func (client *IboxClient) CreateTreeq(ctx context.Context, fsID int, treeqRequest CreateTreeqRequest) (treeq *Treeq, err error) {
	url := fmt.Sprintf("%s%s/%d/treeqs", client.Creds.URL, "api/rest/filesystems", fsID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "fs ID", fsID)

	jsonBytes, err := json.Marshal(treeqRequest)
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

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}

	var responseObject CreateTreeqResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return &responseObject.Result, nil
}

func (client *IboxClient) UpdateTreeq(ctx context.Context, fsID, treeqID int, updateRequest UpdateTreeqRequest) (*Treeq, error) {
	url := fmt.Sprintf("%s%s/%d/treeqs/%d", client.Creds.URL, "api/rest/filesystems", fsID, treeqID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "fs ID", fsID, "treeq ID", treeqID)

	jsonBytes, err := json.Marshal(updateRequest)
	if err != nil {
		return nil, common.Errorf("marshal - error: %w url: %s", err, url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBytes))
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

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}

	var resp UpdateTreeqResponse
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: % url: %ss", err, url)
	}
	if resp.Error.Code != "" {
		if resp.Error.Code == FILESYSTEM_NOT_FOUND || resp.Error.Code == TREEQ_ID_DOES_NOT_EXIST {
			return nil, common.Errorf("errorCode: %s - error: %w url: %s", resp.Error.Code, ErrNotFound, url)
		}
		return nil, common.Errorf("ibox API - error: %v url: %s", resp.Error, url)
	}
	return &resp.Result, nil
}

func (client *IboxClient) GetTreeqsByFileSystem(ctx context.Context, fsID int) (results []Treeq, err error) {
	url := fmt.Sprintf("%s%s/%d/treeqs", client.Creds.URL, "api/rest/filesystems", fsID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "fs ID", fsID)

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
		var responseObject GetTreeqByFileSystemResponse
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
