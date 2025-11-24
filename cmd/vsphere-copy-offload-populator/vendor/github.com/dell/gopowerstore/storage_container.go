/*
 *
 * Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package gopowerstore

import (
	"context"
	"net/http"

	"github.com/dell/gopowerstore/api"
)

const (
	storageContainerURL = "storage_container"
)

func getStorageContainerDefaultQueryParams(c Client) api.QueryParamsEncoder {
	storageContainer := StorageContainer{}
	return c.APIClient().QueryParamsWithFields(&storageContainer)
}

// CreateStorageContainer creates new StorageContainer
func (c *ClientIMPL) CreateStorageContainer(ctx context.Context,
	createParams *StorageContainer,
) (resp CreateResponse, err error) {
	customHeader := http.Header{}
	customHeader.Add("DELL-VISIBILITY", "Partner")
	apiClient := c.APIClient()
	apiClient.SetCustomHTTPHeaders(customHeader)

	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: storageContainerURL,
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// GetStorageContainer get existing StorageContainer with ID
func (c *ClientIMPL) GetStorageContainer(ctx context.Context, id string) (resp StorageContainer, err error) {
	customHeader := http.Header{}
	customHeader.Add("DELL-VISIBILITY", "Partner")
	apiClient := c.APIClient()
	apiClient.SetCustomHTTPHeaders(customHeader)

	_, err = apiClient.Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    storageContainerURL,
			ID:          id,
			QueryParams: getStorageContainerDefaultQueryParams(c),
		},
		&resp)
	return resp, WrapErr(err)
}

// DeleteStorageContainer deletes existing StorageContainer
func (c *ClientIMPL) DeleteStorageContainer(ctx context.Context, id string) (resp EmptyResponse, err error) {
	customHeader := http.Header{}
	customHeader.Add("DELL-VISIBILITY", "Partner")
	apiClient := c.APIClient()
	apiClient.SetCustomHTTPHeaders(customHeader)

	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "DELETE",
			Endpoint: storageContainerURL,
			ID:       id,
			Body:     nil,
		},
		&resp)
	return resp, WrapErr(err)
}

// ModifyStorageContainer updates existing storage container
func (c *ClientIMPL) ModifyStorageContainer(ctx context.Context, modifyParams *StorageContainer, id string) (resp EmptyResponse, err error) {
	customHeader := http.Header{}
	customHeader.Add("DELL-VISIBILITY", "Partner")
	apiClient := c.APIClient()
	apiClient.SetCustomHTTPHeaders(customHeader)

	_, err = apiClient.Query(
		ctx,
		RequestConfig{
			Method:   "PATCH",
			Endpoint: storageContainerURL,
			ID:       id,
			Body:     modifyParams,
		},
		&resp)
	return resp, WrapErr(err)
}
