package iboxapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

func (iboxClient *IboxClient) GetTreeqByName(fsID int, name string) (treeq *Treeq, err error) {
	const functionName = "GetTreeqByName"
	url := fmt.Sprintf("%s%s/%d/treeqs", iboxClient.Creds.URL, "api/rest/filesystems", fsID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "filesystem ID", fsID, "treeq name", name)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := req.URL.Query()
	values.Add("name", name)
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}
	var responseObject GetTreeqByNameResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == FILESYSTEM_NOT_FOUND {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - fs ID '%d' not found", functionName, fsID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}

	if len(responseObject.Result) == 0 {
		return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - treeq %s not found", functionName, name)}
	}
	return &responseObject.Result[0], nil
}

func (iboxClient *IboxClient) GetTreeq(fsID, treeqID int) (treeq *Treeq, err error) {
	const functionName = "GetTreeq"
	url := fmt.Sprintf("%s%s/%d/treeqs/%d", iboxClient.Creds.URL, "api/rest/filesystems", fsID, treeqID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "fs ID", fsID, "treeq ID", treeqID)

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
			iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}
	var responseObject GetTreeqResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == TREEQ_ID_DOES_NOT_EXIST {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - fs ID '%d' treeq ID '%d' not found", functionName, fsID, treeqID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) DeleteTreeq(fsID, treeqID int) (response *Treeq, err error) {
	const functionName = "DeleteTreeq"
	url := fmt.Sprintf("%s%s/%d/treeq/%d", iboxClient.Creds.URL, "api/rest/filesystems", fsID, treeqID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "fs ID", fsID, "treeq ID", treeqID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRquest -  error %w", functionName, err)
	}
	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}

	var responseObject DeleteTreeqResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == TREEQ_ID_DOES_NOT_EXIST {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - fs ID '%d' treeq ID '%d' not found", functionName, fsID, treeqID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}

	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) CreateTreeq(fsID int, treeqRequest CreateTreeqRequest) (treeq *Treeq, err error) {
	const functionName = "CreateTreeq"
	url := fmt.Sprintf("%s%s/%d/treeqs", iboxClient.Creds.URL, "api/rest/filesystems", fsID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "fs ID", fsID)

	jsonBytes, err := json.Marshal(treeqRequest)
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
			iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%s -ReadAll - error %w", functionName, err)
	}

	var responseObject CreateTreeqResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) UpdateTreeq(fsID, treeqID int, updateRequest UpdateTreeqRequest) (*Treeq, error) {
	const functionName = "UpdateTreeq"
	url := fmt.Sprintf("%s%s/%d/treeqs/%d", iboxClient.Creds.URL, "api/rest/filesystems", fsID, treeqID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "fs ID", fsID, "treeq ID", treeqID)

	jsonBytes, err := json.Marshal(updateRequest)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	request, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonBytes))
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
			iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}

	var responseObject UpdateTreeqResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == FILESYSTEM_NOT_FOUND {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s- fs ID '%d' not found", functionName, fsID)}
		}
		if responseObject.Error.Code == TREEQ_ID_DOES_NOT_EXIST {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s- fs ID '%d' treeq ID '%d' treeq does not exist", functionName, fsID, treeqID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) GetTreeqsByFileSystem(fsID int) (results []Treeq, err error) {
	const functionName = "GetTreeqsByFileSystem"
	url := fmt.Sprintf("%s%s/%d/treeqs", iboxClient.Creds.URL, "api/rest/filesystems", fsID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "fs ID", fsID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return results, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
		}

		values := req.URL.Query()
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
				iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return results, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
		}
		var responseObject GetTreeqByFileSystemResponse
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
