package iboxapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/amitosw15/infinibox-csi-driver/common"
	"io"
	"net/http"
	"strconv"
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

func (iboxClient *IboxClient) GetTreeqByName(fsID int, name string) (treeq *Treeq, err error) {
	const function = "GetTreeqByName"
	url := fmt.Sprintf("%s%s/%d/treeqs", iboxClient.Creds.Url, "api/rest/filesystems", fsID)
	iboxClient.Log.V(TRACE_LEVEL).Info(function, "URL", url, "filesystem ID", fsID, "treeq name", name)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", function, err)
	}

	values := req.URL.Query()
	values.Add("name", name)
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", function, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(TRACE_LEVEL).Error(err, function, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", function, err)
	}
	var responseObject GetTreeqByNameResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", function, err)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "FILESYSTEM_NOT_FOUND" {
			return nil, &IboxAPIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - fs ID '%d' not found", function, fsID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", function, responseObject.Error.Code, responseObject.Error.Message)
	}

	if len(responseObject.Result) == 0 {
		return nil, &IboxAPIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - treeq %s not found", function, name)}
	}
	return &responseObject.Result[0], nil
}

func (iboxClient *IboxClient) GetTreeq(fsID, treeqID int) (treeq *Treeq, err error) {
	const function = "GetTreeq"
	url := fmt.Sprintf("%s%s/%d/treeqs/%d", iboxClient.Creds.Url, "api/rest/filesystems", fsID, treeqID)
	iboxClient.Log.V(TRACE_LEVEL).Info(function, "URL", url, "fs ID", fsID, "treeq ID", treeqID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", function, err)
	}
	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", function, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(TRACE_LEVEL).Error(err, function, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", function, err)
	}
	var responseObject GetTreeqResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", function, err)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "TREEQ_ID_DOES_NOT_EXIST" {
			return nil, &IboxAPIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - fs ID '%d' treeq ID '%d' not found", function, fsID, treeqID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", function, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) DeleteTreeq(fsID, treeqID int) (response *Treeq, err error) {
	const function = "DeleteTreeq"
	url := fmt.Sprintf("%s%s/%d/treeq/%d", iboxClient.Creds.Url, "api/rest/filesystems", fsID, treeqID)
	iboxClient.Log.V(TRACE_LEVEL).Info(function, "URL", url, "fs ID", fsID, "treeq ID", treeqID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRquest -  error %w", function, err)
	}
	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", function, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(TRACE_LEVEL).Error(err, function, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", function, err)
	}

	var responseObject DeleteTreeqResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", function, err)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "TREEQ_ID_DOES_NOT_EXIST" {
			return nil, &IboxAPIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - fs ID '%d' treeq ID '%d' not found", function, fsID, treeqID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error code: %s message: %s", function, responseObject.Error.Code, responseObject.Error.Message)
	}

	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) CreateTreeq(fsID int, treeqRequest CreateTreeqRequest) (treeq *Treeq, err error) {

	const function = "CreateTreeq"
	url := fmt.Sprintf("%s%s/%d/treeqs", iboxClient.Creds.Url, "api/rest/filesystems", fsID)
	iboxClient.Log.V(TRACE_LEVEL).Info(function, "URL", url, "fs ID", fsID)

	jsonBytes, err := json.Marshal(treeqRequest)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", function, err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", function, err)
	}
	SetAuthHeader(request, iboxClient.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := iboxClient.HttpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", function, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			iboxClient.Log.V(TRACE_LEVEL).Error(err, function, "error in Close()", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%s -ReadAll - error %w", function, err)
	}

	var responseObject CreateTreeqResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", function, err)
	}

	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error code: %s message: %s", function, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) UpdateTreeq(fsID, treeqID int, updateRequest UpdateTreeqRequest) (*Treeq, error) {
	const function = "UpdateTreeq"
	url := fmt.Sprintf("%s%s/%d/treeqs/%d", iboxClient.Creds.Url, "api/rest/filesystems", fsID, treeqID)
	iboxClient.Log.V(TRACE_LEVEL).Info(function, "URL", url, "fs ID", fsID, "treeq ID", treeqID)

	jsonBytes, err := json.Marshal(updateRequest)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", function, err)
	}
	request, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", function, err)
	}

	SetAuthHeader(request, iboxClient.Creds)

	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := iboxClient.HttpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", function, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			iboxClient.Log.V(TRACE_LEVEL).Error(err, function, "error in Close()", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", function, err)
	}

	var responseObject UpdateTreeqResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", function, err)
	}
	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "FILESYSTEM_NOT_FOUND" {
			return nil, &IboxAPIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s- fs ID '%d' not found", function, fsID)}
		}
		if responseObject.Error.Code == "TREEQ_ID_DOES_NOT_EXIST" {
			return nil, &IboxAPIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s- fs ID '%d' treeq ID '%d' treeq does not exist", function, fsID, treeqID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", function, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) GetTreeqsByFileSystem(fsID int) (results []Treeq, err error) {
	const function = "GetTreeqsByFileSystem"
	url := fmt.Sprintf("%s%s/%d/treeqs", iboxClient.Creds.Url, "api/rest/filesystems", fsID)
	iboxClient.Log.V(TRACE_LEVEL).Info(function, "URL", url, "fs ID", fsID)

	pageSize := common.IBOX_DEFAULT_QUERY_PAGE_SIZE
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(function, "page", page, "totalPages", totalPages)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return results, fmt.Errorf("%s - NewRequest - error %w", function, err)
		}

		values := req.URL.Query()
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()

		SetAuthHeader(req, iboxClient.Creds)

		resp, err := iboxClient.HttpClient.Do(req)
		if err != nil {
			return results, fmt.Errorf("%s - Do - error %w", function, err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				iboxClient.Log.V(TRACE_LEVEL).Error(err, function, "error in Close()", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return results, fmt.Errorf("%s - ReadAll - error %w", function, err)
		}
		var responseObject GetTreeqByFileSystemResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return results, fmt.Errorf("%s - Unmarshal - error %w", function, err)
		}
		results = append(results, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return results, nil
}
