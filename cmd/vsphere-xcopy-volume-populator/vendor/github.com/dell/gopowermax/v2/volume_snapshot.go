/*
 Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

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

package pmax

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	types "github.com/dell/gopowermax/v2/types/v100"

	log "github.com/sirupsen/logrus"
)

// The following constants are for internal use within the pmax library.
const (
	ReplicationX = "replication/"
	PrivateX     = "private/"
	// PrivURLPrefix = RESTPrefix + PrivateX + APIVersion + "/"
	XSnapshot    = "/snapshot"
	XGenereation = "/generation"
)

func (c *Client) privURLPrefix() string {
	return RESTPrefix + PrivateX + c.version + "/"
}

// GetSnapVolumeList returns a list of all snapshot volumes on the array.
func (c *Client) GetSnapVolumeList(ctx context.Context, symID string, queryParams types.QueryParams) (*types.SymVolumeList, error) {
	defer c.TimeSpent("GetSnapVolumeList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.privURLPrefix() + ReplicationX + SymmetrixX + symID + XVolume
	if queryParams != nil {
		URL += "?"
		for key, val := range queryParams {
			switch val := val.(type) {
			case bool:
				URL += fmt.Sprintf("%s=%s", key, strconv.FormatBool(val))
			case string:
				URL += fmt.Sprintf("%s=%s", key, val)
			}
			URL += "&"
		}
		URL = URL[:len(URL)-1]
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetSnapVolumeList failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	snapVolList := &types.SymVolumeList{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(snapVolList); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return snapVolList, nil
}

// GetVolumeSnapInfo returns snapVx information associated with a volume.
func (c *Client) GetVolumeSnapInfo(ctx context.Context, symID string, volumeID string) (*types.SnapshotVolumeGeneration, error) {
	defer c.TimeSpent("GetVolumeSnapInfo", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.privURLPrefix() + ReplicationX + SymmetrixX + symID + XVolume + "/" + volumeID + XSnapshot
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetVolumeSnapInfo failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	snapinfo := &types.SnapshotVolumeGeneration{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(snapinfo); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return snapinfo, nil
}

// GetSnapshotInfo returns snapVx information of the specified snapshot
func (c *Client) GetSnapshotInfo(ctx context.Context, symID, volumeID, snapID string) (*types.VolumeSnapshot, error) {
	defer c.TimeSpent("GetSnapshotInfo", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.privURLPrefix() + ReplicationX + SymmetrixX + symID + XVolume + "/" + volumeID + XSnapshot + "/" + snapID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetSnapshotInfo failed: " + err.Error())
		return nil, err
	}
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	snapshotInfo := new(types.VolumeSnapshot)
	if err := json.NewDecoder(resp.Body).Decode(snapshotInfo); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return snapshotInfo, nil
}

// CreateSnapshot creates a snapVx snapshot of a volume or on the list of volumes passed as sourceVolumeList
//  BothSides flag is used in SRDF usecases to create snapshots on both R1 and R2 side
//  Star flag is used if the source device is participating in SRDF star mode
//  Use the Force flag to automate some scenarios to succeed
//  TimeToLive value ins hour is set on the snapshot to automatically delete the snapshot after target is unlinked
func (c *Client) CreateSnapshot(ctx context.Context, symID string, snapID string, sourceVolumeList []types.VolumeList, ttl int64) error {
	defer c.TimeSpent("CreateSnapshot", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	snapParam := &types.CreateVolumesSnapshot{
		SourceVolumeList: sourceVolumeList,
		BothSides:        false,
		Star:             false,
		Force:            false,
		TimeToLive:       ttl,
		ExecutionOption:  types.ExecutionOptionSynchronous,
	}
	ifDebugLogPayload(snapParam)
	URL := c.privURLPrefix() + ReplicationX + SymmetrixX + symID + XSnapshot + "/" + snapID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Post(ctx, URL, c.getDefaultHeaders(), snapParam, nil)
	if err != nil {
		log.Error("CreateSnapshot failed: " + err.Error())
	}
	return err
}

// DeleteSnapshot deletes a snapshot from a volume
// DeviceNameListSource is a list which contains the names of source volumes
// Symforce flag is used to automate some internal establish scenarios
//  Star mode is used for devices in SRDF relations
//  Use the Force flag in acceptable error conditions
// Restore, when set to true will terminate the Restore and the Snapshot as well
//  Generation is used to tell which generation of snapshot needs to be deleted and is passed as int64
// ExecutionOption tells the Unisphere to perform the operation either in Synchronous mode or Asynchronous mode
func (c *Client) DeleteSnapshot(ctx context.Context, symID, snapID string, sourceVolumes []types.VolumeList, generation int64) error {
	defer c.TimeSpent("DeleteSnapshot", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	deleteSnapshot := &types.DeleteVolumeSnapshot{
		DeviceNameListSource: sourceVolumes,
		Symforce:             false,
		Star:                 false,
		Force:                false,
		Restore:              false,
		Generation:           generation,
		ExecutionOption:      types.ExecutionOptionAsynchronous,
	}
	job := &types.Job{}
	ifDebugLogPayload(deleteSnapshot)
	URL := c.privURLPrefix() + ReplicationX + SymmetrixX + symID + XSnapshot + "/" + snapID
	URL = strings.Replace(URL, "/90/", "/91/", 1)
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.DoWithHeaders(ctx, http.MethodDelete, URL, c.getDefaultHeaders(), deleteSnapshot, job)
	if err != nil {
		return err
	}
	job, err = c.WaitOnJobCompletion(ctx, symID, job.JobID)
	if err != nil {
		return err
	}
	if job.Status == types.JobStatusFailed || job.Status == types.JobStatusRunning {
		return fmt.Errorf("Job status not successful for snapshot delete. Job status = %s and Job result = %s", job.Status, job.Result)
	}
	log.Info(fmt.Sprintf("Snapshot (%s) deleted successfully", snapID))
	return nil
}

// DeleteSnapshotS - Deletes a snapshot synchronously
func (c *Client) DeleteSnapshotS(ctx context.Context, symID, snapID string, sourceVolumes []types.VolumeList, generation int64) error {
	defer c.TimeSpent("DeleteSnapshotS", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	deleteSnapshot := &types.DeleteVolumeSnapshot{
		DeviceNameListSource: sourceVolumes,
		Symforce:             false,
		Star:                 false,
		Force:                false,
		Restore:              false,
		Generation:           generation,
		ExecutionOption:      types.ExecutionOptionSynchronous,
	}
	URL := c.privURLPrefix() + ReplicationX + SymmetrixX + symID + XSnapshot + "/" + snapID
	URL = strings.Replace(URL, "/90/", "/91/", 1)
	fields := map[string]interface{}{
		http.MethodPut: URL,
	}
	ifDebugLogPayload(deleteSnapshot)
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.DoWithHeaders(ctx, http.MethodDelete, URL, c.getDefaultHeaders(), deleteSnapshot, nil)
	if err != nil {
		log.WithFields(fields).Errorf("Delete Snapshot (%s:%s) failed with error: %s", symID, snapID, err.Error())
		return err
	}
	log.Info(fmt.Sprintf("Snapshot (%s) deleted successfully", snapID))
	return nil
}

// ModifySnapshot executes actions on a snapshot
// VolumeNameListSource is a list which contains the names of source volumes
// VolumeNameListTarget is a list which contains the names of target volumes to which the snapshot is linked or going to be linked
// Symforce flag is used to automate some internal establish scenarios
// Star mode is used for devices in SRDF relations
// Use the Force flag in acceptable error conditions
// Restore, when set to true will terminate the Restore and the Snapshot as well
// Exact when specified, pairs source and link devices in their ordinal positions within the selection. When not set uses the source and link device selections as a pool that pairs by best match
// Copy when specified creates an exact copy of the source device, otherwise copies the references
// Remote when specified propagates the data to the remote mirror of the RDF device
// Generation is used to tell which generation of snapshot needs to be updated, it is passed as int64
// NewSnapshotName specifies the new snapshot name to which the old snapshot will be renamed
// ExecutionOption tells the Unisphere to perform the operation either in Synchronous mode or Asynchronous mode
// Action defined the operation which will be performed on the given snapshot
func (c *Client) ModifySnapshot(ctx context.Context, symID string, sourceVol []types.VolumeList,
	targetVol []types.VolumeList, snapID string, action string,
	newSnapID string, generation int64, isCopy bool,
) error {
	defer c.TimeSpent("ModifySnapshot", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}

	snapParam := &types.ModifyVolumeSnapshot{}

	switch action {
	case "Link":
		snapParam = &types.ModifyVolumeSnapshot{
			VolumeNameListSource: sourceVol,
			VolumeNameListTarget: targetVol,
			Force:                false,
			Star:                 false,
			Exact:                false,
			Copy:                 isCopy,
			Remote:               false,
			Symforce:             false,
			Action:               action,
			Generation:           generation,
			ExecutionOption:      types.ExecutionOptionAsynchronous,
		}
	case "Unlink":
		snapParam = &types.ModifyVolumeSnapshot{
			VolumeNameListSource: sourceVol,
			VolumeNameListTarget: targetVol,
			Force:                false,
			Star:                 false,
			Symforce:             false,
			Action:               action,
			Generation:           generation,
			ExecutionOption:      types.ExecutionOptionAsynchronous,
		}
	case "Rename":
		snapParam = &types.ModifyVolumeSnapshot{
			VolumeNameListSource: sourceVol,
			VolumeNameListTarget: targetVol,
			NewSnapshotName:      newSnapID,
			Action:               action,
			ExecutionOption:      types.ExecutionOptionAsynchronous,
		}
	default:
		return fmt.Errorf("not a supported action on Snapshots")
	}
	URL := c.privURLPrefix() + ReplicationX + SymmetrixX + symID + XSnapshot + "/" + snapID
	job := &types.Job{}
	fields := map[string]interface{}{
		http.MethodPut: URL,
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), snapParam, job)
	if err != nil {
		log.WithFields(fields).Error("Error in ModifySnapshot: " + err.Error())
		return err
	}
	job, err = c.WaitOnJobCompletion(ctx, symID, job.JobID)
	if err != nil {
		return err
	}
	if job.Status == types.JobStatusFailed || job.Status == types.JobStatusRunning {
		return fmt.Errorf("Job status not successful for snapshot %s. Job status = %s and Job result = %s", action, job.Status, job.Result)
	}
	log.Info(fmt.Sprintf("Action (%s) on Snapshot (%s) is successful", action, snapID))
	return nil
}

// ModifySnapshotS executes actions on snapshots synchronously
func (c *Client) ModifySnapshotS(ctx context.Context, symID string, sourceVol []types.VolumeList,
	targetVol []types.VolumeList, snapID string, action string,
	newSnapID string, generation int64, isCopy bool,
) error {
	defer c.TimeSpent("ModifySnapshotS", time.Now())

	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}

	snapParam := &types.ModifyVolumeSnapshot{}

	switch action {
	case "Link":
		snapParam = &types.ModifyVolumeSnapshot{
			VolumeNameListSource: sourceVol,
			VolumeNameListTarget: targetVol,
			Force:                false,
			Star:                 false,
			Exact:                false,
			Copy:                 isCopy,
			Remote:               false,
			Symforce:             false,
			Action:               action,
			Generation:           generation,
			ExecutionOption:      types.ExecutionOptionSynchronous,
		}
	case "Unlink":
		snapParam = &types.ModifyVolumeSnapshot{
			VolumeNameListSource: sourceVol,
			VolumeNameListTarget: targetVol,
			Force:                false,
			Star:                 false,
			Symforce:             false,
			Action:               action,
			Generation:           generation,
			ExecutionOption:      types.ExecutionOptionSynchronous,
		}
	case "Rename":
		snapParam = &types.ModifyVolumeSnapshot{
			VolumeNameListSource: sourceVol,
			VolumeNameListTarget: targetVol,
			NewSnapshotName:      newSnapID,
			Action:               action,
			ExecutionOption:      types.ExecutionOptionSynchronous,
		}
	default:
		return fmt.Errorf("not a supported action on Snapshots")
	}
	URL := c.privURLPrefix() + ReplicationX + SymmetrixX + symID + XSnapshot + "/" + snapID
	fields := map[string]interface{}{
		http.MethodPut: URL,
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(ctx, URL, c.getDefaultHeaders(), snapParam, nil)
	if err != nil {
		log.WithFields(fields).Error("Error in ModifySnapshotS: " + err.Error())
		return err
	}
	log.Info(fmt.Sprintf("Action (%s) on Snapshot (%s) is successful", action, snapID))
	return nil
}

// GetPrivVolumeByID returns a Volume structure given the symmetrix and volume ID
func (c *Client) GetPrivVolumeByID(ctx context.Context, symID string, volumeID string) (*types.VolumeResultPrivate, error) {
	defer c.TimeSpent("GetPrivVolumeByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	vol, err := c.GetVolumeByID(ctx, symID, volumeID)
	if err != nil {
		log.Error("GetVolumeByID failed: " + err.Error())
		return nil, err
	}

	wwn := vol.WWN
	URL := c.privURLPrefix() + SLOProvisioningX + SymmetrixX + symID + XVolume
	URL = fmt.Sprintf("%s?wwn=%s", URL, wwn)
	// URL = URL + query

	ctx, cancel := context.WithTimeout(ctx, 360*time.Second)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetPrivVolumeByID failed: " + err.Error())
		return nil, err
	}
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	// volume := &types.VolumeResultPrivate{}
	privateVolumeIterator := new(types.PrivVolumeIterator)
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(privateVolumeIterator); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return &privateVolumeIterator.ResultList.PrivVolumeList[0], nil
}

// GetSnapshotGenerations returns a list of all the snapshot generation on a specific snapshot
func (c *Client) GetSnapshotGenerations(ctx context.Context, symID, volumeID, snapID string) (*types.VolumeSnapshotGenerations, error) {
	defer c.TimeSpent("GetSnapshotGenerations", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.privURLPrefix() + ReplicationX + SymmetrixX + symID + XVolume + "/" + volumeID + XSnapshot + "/" + snapID + XGenereation
	volumeSnapshotGenerations := new(types.VolumeSnapshotGenerations)
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), volumeSnapshotGenerations)
	if err != nil {
		return nil, err
	}
	return volumeSnapshotGenerations, nil
}

// GetSnapshotGenerationInfo returns the specific generation info related to a snapshot
func (c *Client) GetSnapshotGenerationInfo(ctx context.Context, symID, volumeID, snapID string, generation int64) (*types.VolumeSnapshotGeneration, error) {
	defer c.TimeSpent("GetSnapshotGenerationInfo", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.privURLPrefix() + ReplicationX + SymmetrixX + symID + XVolume + "/" + volumeID + XSnapshot + "/" + snapID + XGenereation + "/" + strconv.FormatInt(generation, 10)
	volumeSnapshotGeneration := new(types.VolumeSnapshotGeneration)
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), volumeSnapshotGeneration)
	if err != nil {
		return nil, err
	}
	return volumeSnapshotGeneration, nil
}

// GetReplicationCapabilities returns details about SnapVX and SRDF
// execution capabilities on the Symmetrix array
func (c *Client) GetReplicationCapabilities(ctx context.Context) (*types.SymReplicationCapabilities, error) {
	defer c.TimeSpent("GetReplicationCapabilities", time.Now())
	URL := c.urlPrefix() + ReplicationX + "capabilities/symmetrix"
	symReplicationCapabilities := new(types.SymReplicationCapabilities)
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), symReplicationCapabilities)
	if err != nil {
		return nil, err
	}
	return symReplicationCapabilities, nil
}
