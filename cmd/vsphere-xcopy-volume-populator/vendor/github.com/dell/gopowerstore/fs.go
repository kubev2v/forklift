/*
 *
 * Copyright © 2020-2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"fmt"

	"github.com/dell/gopowerstore/api"
	log "github.com/sirupsen/logrus"
)

const (
	nasURL = "nas_server"
	fsURL  = "file_system"
)

func getNASDefaultQueryParams(c Client) api.QueryParamsEncoder {
	nas := NAS{}
	return c.APIClient().QueryParamsWithFields(&nas)
}

func getFSDefaultQueryParams(c Client) api.QueryParamsEncoder {
	fs := FileSystem{}
	return c.APIClient().QueryParamsWithFields(&fs)
}

func getNfsServerDefaultQueryParams(c Client) api.QueryParamsEncoder {
	nfsServer := NFSServerInstance{}
	return c.APIClient().QueryParamsWithFields(&nfsServer)
}

// GetNASServers query and return all NAS servers
func (c *ClientIMPL) GetNASServers(ctx context.Context) ([]NAS, error) {
	var result []NAS
	var qp api.QueryParamsEncoder
	var fields []string
	err := c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []NAS
		arrayVerion, err := c.GetSoftwareMajorMinorVersion(ctx)
		if err != nil {
			log.Errorf("Couldn't find the array version %s", err.Error())
		}

		fields = GetNASFields(arrayVerion)
		qp = c.APIClient().QueryParams().Select(fields...)

		qp.Offset(offset).Limit(paginationDefaultPageSize)
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    nasURL,
				QueryParams: qp,
			},
			&page)
		err = WrapErr(err)
		if err == nil {
			result = append(result, page...)
		}
		return meta, err
	})
	return result, err
}

// GetNASByName query and return specific NAS by name
func (c *ClientIMPL) GetNASByName(ctx context.Context, name string) (resp NAS, err error) {
	var nasList []NAS
	var qp api.QueryParamsEncoder
	var fields []string
	arrayVerion, err := c.GetSoftwareMajorMinorVersion(ctx)
	if err != nil {
		log.Errorf("Couldn't find the array version %s", err.Error())
	}

	fields = GetNASFields(arrayVerion)
	qp = c.APIClient().QueryParams().Select(fields...)

	qp.RawArg("name", fmt.Sprintf("eq.%s", name))
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    nasURL,
			QueryParams: qp,
		},
		&nasList)
	err = WrapErr(err)
	if err != nil {
		return resp, err
	}
	if len(nasList) != 1 {
		return resp, NewNotFoundError()
	}
	return nasList[0], err
}

// GetNAS query and return specific NAS by id
func (c *ClientIMPL) GetNAS(ctx context.Context, id string) (resp NAS, err error) {
	var qp api.QueryParamsEncoder
	var fields []string
	arrayVerion, err := c.GetSoftwareMajorMinorVersion(ctx)
	if err != nil {
		log.Errorf("Couldn't find the array version %s", err.Error())
	}

	fields = GetNASFields(arrayVerion)
	qp = c.APIClient().QueryParams().Select(fields...)

	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    nasURL,
			ID:          id,
			QueryParams: qp,
		},
		&resp)
	return resp, WrapErr(err)
}

// GetNfsServer query and return specified NFS server instance by id
func (c *ClientIMPL) GetNfsServer(ctx context.Context, id string) (resp NFSServerInstance, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    nfsServerURL,
			ID:          id,
			QueryParams: getNfsServerDefaultQueryParams(c),
		},
		&resp)
	return resp, WrapErr(err)
}

// CreateNAS creates new NAS on storage array
func (c *ClientIMPL) CreateNAS(ctx context.Context, createParams *NASCreate) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: nasURL,
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// DeleteNAS deletes existing NAS
func (c *ClientIMPL) DeleteNAS(ctx context.Context, id string) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "DELETE",
			Endpoint: nasURL,
			ID:       id,
		},
		&resp)
	return resp, WrapErr(err)
}

// GetFSByName query and return specific FS by name
func (c *ClientIMPL) GetFSByName(ctx context.Context, name string) (resp FileSystem, err error) {
	var fsList []FileSystem
	qp := getFSDefaultQueryParams(c)
	qp.RawArg("name", fmt.Sprintf("eq.%s", name))
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    fsURL,
			QueryParams: qp,
		},
		&fsList)
	err = WrapErr(err)
	if err != nil {
		return resp, err
	}
	if len(fsList) != 1 {
		return resp, NewNotFoundError()
	}
	return fsList[0], err
}

// GetFS query and return specific fs by id
func (c *ClientIMPL) GetFS(ctx context.Context, id string) (resp FileSystem, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    fsURL,
			ID:          id,
			QueryParams: getFSDefaultQueryParams(c),
		},
		&resp)
	return resp, WrapErr(err)
}

// CreateFS creates new filesystem on storage array
func (c *ClientIMPL) CreateFS(ctx context.Context, createParams *FsCreate) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: fsURL,
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// DeleteFS deletes existing filesystem
func (c *ClientIMPL) DeleteFS(ctx context.Context, id string) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "DELETE",
			Endpoint: fsURL,
			ID:       id,
		},
		&resp)
	return resp, WrapErr(err)
}

// CreateSnapshot creates a new snapshot
func (c *ClientIMPL) CreateFsSnapshot(ctx context.Context,
	createSnapFSParams *SnapshotFSCreate, id string,
) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: fsURL,
			ID:       id,
			Action:   "snapshot",
			Body:     createSnapFSParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// DeleteFsSnapshot is an alias for delete filesystem, because snapshots are essentially just filesystems
func (c *ClientIMPL) DeleteFsSnapshot(ctx context.Context, id string) (resp EmptyResponse, err error) {
	return c.DeleteFS(ctx, id)
}

// GetFsSnapshot query and return specific fs snapshot by it's id
func (c *ClientIMPL) GetFsSnapshot(ctx context.Context, snapID string) (resVol FileSystem, err error) {
	qp := getFSDefaultQueryParams(c)
	qp.RawArg("filesystem_type", fmt.Sprintf("eq.%s", FileSystemTypeEnumSnapshot))
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    fsURL,
			ID:          snapID,
			QueryParams: qp,
		},
		&resVol)
	return resVol, WrapErr(err)
}

// GetFsSnapshots returns all fs snapshots
func (c *ClientIMPL) GetFsSnapshots(ctx context.Context) ([]FileSystem, error) {
	var result []FileSystem
	err := c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []FileSystem
		qp := getFSDefaultQueryParams(c)
		qp.RawArg("filesystem_type", fmt.Sprintf("eq.%s", FileSystemTypeEnumSnapshot))
		qp.Order("name")
		qp.Offset(offset).Limit(paginationDefaultPageSize)
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    fsURL,
				QueryParams: qp,
			},
			&page)
		err = WrapErr(err)
		if err == nil {
			result = append(result, page...)
		}
		return meta, err
	})
	return result, err
}

// GetFsSnapshotsByVolumeID returns a list of fs snapshots for specific volume
func (c *ClientIMPL) GetFsSnapshotsByVolumeID(ctx context.Context, volID string) ([]FileSystem, error) {
	var result []FileSystem
	err := c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []FileSystem
		qp := getFSDefaultQueryParams(c)
		qp.RawArg("parent_id", fmt.Sprintf("eq.%s", volID))
		qp.RawArg("filesystem_type", fmt.Sprintf("eq.%s", FileSystemTypeEnumSnapshot))
		qp.Order("name")
		qp.Offset(offset).Limit(paginationDefaultPageSize)
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    fsURL,
				QueryParams: qp,
			},
			&page)
		err = WrapErr(err)
		if err == nil {
			result = append(result, page...)
		}
		return meta, err
	})
	return result, err
}

func (c *ClientIMPL) ModifyFS(ctx context.Context,
	modifyParams *FSModify, id string,
) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "PATCH",
			Endpoint: fsURL,
			ID:       id,
			Body:     modifyParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// CreateFsFromSnapshot creates a new fs by cloning a snapshot
func (c *ClientIMPL) CreateFsFromSnapshot(ctx context.Context,
	createParams *FsClone, snapID string,
) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: fsURL,
			ID:       snapID,
			Action:   "clone",
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// CloneFS creates a new fs by cloning a existing fs
func (c *ClientIMPL) CloneFS(ctx context.Context,
	createParams *FsClone, fsID string,
) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: fsURL,
			ID:       fsID,
			Action:   "clone",
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

func (c *ClientIMPL) GetFsByFilter(ctx context.Context, filter map[string]string) ([]FileSystem, error) {
	var result []FileSystem
	err := c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []FileSystem
		qp := getFSDefaultQueryParams(c)
		for k, v := range filter {
			qp.RawArg(k, v)
		}
		qp.Offset(offset).Limit(paginationDefaultPageSize)
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    fsURL,
				QueryParams: qp,
			},
			&page)
		err = WrapErr(err)
		if err == nil {
			result = append(result, page...)
		}
		return meta, err
	})
	return result, err
}

func GetNASFields(arrayVerion float32) []string {
	var fields []string
	fields = []string{"id", "description", "name", "current_node_id", "operational_status", "current_preferred_IPv4_interface_id", "current_preferred_IPv6_interface_id", "nfs_servers", "file_systems", "health_details", "preferred_node_id", "default_unix_user", "default_windows_user", "current_unix_directory_service", "is_username_translation_enabled", "is_auto_user_mapping_enabled", "production_IPv4_interface_id", "production_IPv6_interface_id", "backup_IPv4_interface_id", "backup_IPv6_interface_id", "protection_policy_id", "file_events_publishing_mode", "is_replication_destination", "is_production_mode_enabled", "operational_status_l10n", "current_unix_directory_service_l10n", "file_events_publishing_mode_l10n"}

	if arrayVerion > 3.6 {
		fields = append(fields, "is_dr_test")
	}

	return fields
}
