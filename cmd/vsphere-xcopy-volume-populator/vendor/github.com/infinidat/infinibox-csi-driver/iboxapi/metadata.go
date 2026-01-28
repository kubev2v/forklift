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

type MetadataResult struct {
	ID         int    `json:"id"`
	ObjectID   int    `json:"object_id"`
	Key        string `json:"key"`
	Value      string `json:"value"`
	ObjectType string `json:"object_type"`
}

type DeleteMetadataResponse struct {
	Results  []MetadataResult `json:"results"`
	Error    Error            `json:"error"`
	Metadata Metadata         `json:"metadata"`
}

type PutMetadataResponse struct {
	Results  []MetadataResult `json:"results"`
	Error    Error            `json:"error"`
	Metadata Metadata         `json:"metadata"`
}

type GetMetadataResponse struct {
	Metadata Metadata            `json:"metadata"`
	Result   []GetMetadataResult `json:"result"`
	Error    any                 `json:"error"`
}
type GetMetadataResult struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	ObjectType string `json:"object_type"`
	ID         int    `json:"id"`
	ObjectID   int    `json:"object_id"`
}

func (client *IboxClient) PutMetadata(ctx context.Context, objectID int, metadata map[string]any) (r *PutMetadataResponse, err error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/metadata/", objectID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "object ID", objectID, "map", metadata)

	jsonBytes, err := json.Marshal(metadata)
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

	var responseObject PutMetadataResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - errorCode: %s message: %s url: %s", responseObject.Error.Code, responseObject.Error.Message, url)
	}
	return &responseObject, nil
}

func (client *IboxClient) GetMetadata(ctx context.Context, objectID int) (results []GetMetadataResult, err error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/metadata", objectID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "object ID", objectID)

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
		var responseObject GetMetadataResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return results, common.Errorf("unmarshal - error: %w url: %s", err, url)
		}
		slog.Log(ctx, common.LevelTrace, "info", "resp", responseObject)
		results = append(results, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return results, nil
}

func (client *IboxClient) DeleteMetadata(ctx context.Context, objectID int) (response *DeleteMetadataResponse, err error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/metadata", objectID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "object ID", objectID)

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
	var responseObject DeleteMetadataResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - errorCode: %s message: %s url: %s", responseObject.Error.Code, responseObject.Error.Message, url)
	}
	return &responseObject, nil
}
