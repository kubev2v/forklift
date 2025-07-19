/*
 Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.

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
	"time"

	types "github.com/dell/gopowermax/v2/types/v100"
	log "github.com/sirupsen/logrus"
)

// The follow constants are for internal use within the pmax library.
const (
	Replication = "replication/"
	SnapID      = "/snapid"
)

// SnapshotAction A list of possible Snapshot actions.
type SnapshotAction string

// Snapshot Actions
const (
	Restore       SnapshotAction = "Restore"       // Restores a snapshot generation
	SetTimeToLive SnapshotAction = "SetTimeToLive" // Set the number of days or hours for a snapshot generation before it auto-terminates
	SetSecure     SnapshotAction = "SetSecure"     // Set the number of days or hours for a snapshot generation to be secure before it auto-terminates
	Link          SnapshotAction = "Link"          // Link a snapshot generation
	Relink        SnapshotAction = "Relink"        // Relink a snapshot generation
	Unlink        SnapshotAction = "Unlink"        // Unlink a snapshot generation
	SetMode       SnapshotAction = "SetMode"       // Set the mode of a linked snapshot generation
	Rename        SnapshotAction = "Rename"        // Rename a snapshot
	Persist       SnapshotAction = "Persist"       // Persist a snapshot policy snapshot
)

// GetStorageGroupSnapshots Get All Storage Group Snapshots
func (c *Client) GetStorageGroupSnapshots(ctx context.Context, symID string, storageGroupID string, excludeManualSnaps bool, excludeSlSnaps bool) (*types.StorageGroupSnapshot, error) {
	defer c.TimeSpent("GetStorageGroupSnapshots", time.Now())
	query := ""
	if excludeManualSnaps && excludeSlSnaps {
		query = "?exclude_manual_snaps=true&exclude_sl_snaps=true"
	} else if excludeManualSnaps {
		query = "?exclude_manual_snap=true"
	} else if excludeSlSnaps {
		query = "?exclude_sl_snaps=true"
	}

	URL := c.urlPrefix() + Replication + SymmetrixX + symID + XStorageGroup + "/" + storageGroupID + XSnapshot + query

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetStorageGroupSnapshots failed: " + err.Error())
		return nil, err
	}
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	snapshots := &types.StorageGroupSnapshot{}
	decoder := json.NewDecoder(resp.Body)

	if err = decoder.Decode(snapshots); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return snapshots, nil
}

// GetStorageGroupSnapshotSnapIDs Get a list of SnapIDs for a particular snapshot
func (c *Client) GetStorageGroupSnapshotSnapIDs(ctx context.Context, symID string, storageGroupID string, snapshotID string) (*types.SnapID, error) {
	defer c.TimeSpent("GetStorageGroupSnapshotSnapIDs", time.Now())
	URL := c.urlPrefix() + Replication + SymmetrixX + symID + XStorageGroup + "/" + storageGroupID + XSnapshot + "/" + snapshotID + SnapID

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetStorageGroupSnapshotSnapIDs failed: " + err.Error())
		return nil, err
	}
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	snapids := &types.SnapID{}
	decoder := json.NewDecoder(resp.Body)

	if err = decoder.Decode(snapids); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully Fetched Snapids for StorageGroup %s", storageGroupID))
	return snapids, nil
}

// GetStorageGroupSnapshotSnap Get the details of a storage group snapshot snap
func (c *Client) GetStorageGroupSnapshotSnap(ctx context.Context, symID string, storageGroupID string, snapshotID, snapID string) (*types.StorageGroupSnap, error) {
	defer c.TimeSpent("GetStorageGroupSnapshotSnapIDs", time.Now())
	URL := c.urlPrefix() + Replication + SymmetrixX + symID + XStorageGroup + "/" + storageGroupID + XSnapshot + "/" + snapshotID + SnapID + "/" + snapID

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetStorageGroupSnapshotSnapIDs failed: " + err.Error())
		return nil, err
	}
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	snap := &types.StorageGroupSnap{}
	decoder := json.NewDecoder(resp.Body)

	if err = decoder.Decode(snap); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully Fetched Snapids for StorageGroup %s", storageGroupID))
	return snap, nil
}

// CreateStorageGroupSnapshot Create a Storage Group Snapshot
func (c *Client) CreateStorageGroupSnapshot(ctx context.Context, symID string, storageGroupID string, payload *types.CreateStorageGroupSnapshot) (*types.StorageGroupSnap, error) {
	defer c.TimeSpent("CreateStorageGroupSnapshot", time.Now())
	ifDebugLogPayload(payload)
	URL := c.urlPrefix() + Replication + SymmetrixX + symID + XStorageGroup + "/" + storageGroupID + XSnapshot
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	snap := &types.StorageGroupSnap{}
	err := c.api.Post(ctx, URL, c.getDefaultHeaders(), payload, snap)
	if err != nil {
		log.Error("CreateStorageGroupSnapshot failed: " + err.Error())
		return nil, err
	}
	log.Info("Successfully created CreateStorageGroupSnapshot")
	return snap, nil
}

// ModifyStorageGroupSnapshot Modify a Storage Group Snapshot snap
func (c *Client) ModifyStorageGroupSnapshot(ctx context.Context, symID string, storageGroupID string, snapshotID string, snapID string, payload *types.ModifyStorageGroupSnapshot) (*types.StorageGroupSnap, error) {
	defer c.TimeSpent("ModifyStorageGroupSnapshot", time.Now())
	URL := c.urlPrefix() + Replication + SymmetrixX + symID + XStorageGroup + "/" + storageGroupID + XSnapshot + "/" + snapshotID + SnapID + "/" + snapID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	snap := &types.StorageGroupSnap{}

	ifDebugLogPayload(payload)
	var putPayload interface{}
	switch payload.Action {
	case string(Restore):
		putPayload = &types.RestoreStorageGroupSnapshot{
			Action:          payload.Action,
			ExecutionOption: payload.ExecutionOption,
			Restore:         payload.Restore,
		}
	case string(Link):
		putPayload = &types.LinkStorageGroupSnapshot{
			Action:          payload.Action,
			ExecutionOption: payload.ExecutionOption,
			Link:            payload.Link,
		}
	case string(Relink):
		putPayload = &types.RelinkStorageGroupSnapshot{
			Action:          payload.Action,
			ExecutionOption: payload.ExecutionOption,
			Relink:          payload.Relink,
		}
	case string(Unlink):
		putPayload = &types.UnlinkStorageGroupSnapshot{
			Action:          payload.Action,
			ExecutionOption: payload.ExecutionOption,
			Unlink:          payload.Unlink,
		}
	case string(SetMode):
		putPayload = &types.SetModeStorageGroupSnapshot{
			Action:          payload.Action,
			ExecutionOption: payload.ExecutionOption,
			SetMode:         payload.SetMode,
		}
	case string(Rename):
		putPayload = &types.RenameStorageGroupSnapshot{
			Action:          payload.Action,
			ExecutionOption: payload.ExecutionOption,
			Rename:          payload.Rename,
		}
	case string(SetTimeToLive):
		putPayload = &types.TimeToLiveStorageGroupSnapshot{
			Action:          payload.Action,
			ExecutionOption: payload.ExecutionOption,
			TimeToLive:      payload.TimeToLive,
		}
	case string(SetSecure):
		putPayload = &types.SecureStorageGroupSnapshot{
			Action:          payload.Action,
			ExecutionOption: payload.ExecutionOption,
			Secure:          payload.Secure,
		}
	case string(Persist):
		putPayload = &types.PersistStorageGroupSnapshot{
			Action:          payload.Action,
			ExecutionOption: payload.ExecutionOption,
			Persist:         payload.Persist,
		}
	}
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), putPayload, snap)
	if err != nil {
		log.Error("ModifyStorageGroupSnapshot failed: " + err.Error())
		return nil, err
	}
	log.Info("Successfully created ModifyStorageGroupSnapshot")
	return snap, nil
}

// DeleteStorageGroupSnapshot Delete a Storage Group Snapshot snap
func (c *Client) DeleteStorageGroupSnapshot(ctx context.Context, symID string, storageGroupID string, snapshotID string, snapID string) error {
	defer c.TimeSpent("DeleteStorageGroupSnapshot", time.Now())
	URL := c.urlPrefix() + Replication + SymmetrixX + symID + XStorageGroup + "/" + storageGroupID + XSnapshot + "/" + snapshotID + SnapID + "/" + snapID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Delete(ctx, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("Error in Delete Storage Group Snapshot: " + err.Error())
	} else {
		log.Infof("Successfully deleted Storage Group Snapshot: %s", snapID)
	}
	return err
}

// GetSnapshotPolicy returns a SnapshotPolicy given the Symmetrix ID and SnapshotPolicy ID (which is really a name).
func (c *Client) GetSnapshotPolicy(ctx context.Context, symID string, snapshotPolicyID string) (*types.SnapshotPolicy, error) {
	defer c.TimeSpent("GetSnapshotPolicy", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + Replication + SymmetrixX + symID + SnapshotPolicy + "/" + snapshotPolicyID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetSnapshotPolicy failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	snapshotPolicy := &types.SnapshotPolicy{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(snapshotPolicy); err != nil {
		return nil, err
	}

	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return snapshotPolicy, nil
}

// DeleteSnapshotPolicy deletes a SnapshotPolicy entry.
func (c *Client) DeleteSnapshotPolicy(ctx context.Context, symID string, snapshotPolicyID string) error {
	defer c.TimeSpent("DeleteSnapshotPolicy", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	URL := c.urlPrefix() + Replication + SymmetrixX + symID + SnapshotPolicy + "/" + snapshotPolicyID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Delete(ctx, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("DeleteSnapshotPolicy failed: " + err.Error())
		return err
	}
	log.Info(fmt.Sprintf("Successfully deleted SnapshotPolicy: %s", snapshotPolicyID))
	return nil
}

// CreateSnapshotPolicy creates a Snapshot policy and returns a types.SnapshotPolicy.
func (c *Client) CreateSnapshotPolicy(ctx context.Context, symID string, snapshotPolicyID string, interval string, offsetMins int32, complianceCountWarn int64,
	complianceCountCritical int64, optionalPayload map[string]interface{},
) (*types.SnapshotPolicy, error) {
	defer c.TimeSpent("CreateSnapshotPolicy", time.Now())

	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	snapshotPolicyParam := &types.CreateSnapshotPolicyParam{
		SnapshotPolicyName:      snapshotPolicyID,
		ExecutionOption:         types.ExecutionOptionSynchronous,
		Interval:                interval,
		OffsetMins:              offsetMins,
		ComplianceCountWarning:  complianceCountWarn,
		ComplianceCountCritical: complianceCountCritical,
	}

	if len(optionalPayload) > 0 {
		cloudSnapshotPolicyDetails, okCloud := optionalPayload["cloudSnapshotPolicyDetails"]
		if okCloud {
			snapshotPolicyParam.CloudSnapshotPolicyDetails = cloudSnapshotPolicyDetails.(*types.CloudSnapshotPolicyDetails)
		}
		localSnapshotPolicyDetails, okLocal := optionalPayload["localSnapshotPolicyDetails"]
		if okLocal {
			snapshotPolicyParam.LocalSnapshotPolicyDetails = localSnapshotPolicyDetails.(*types.LocalSnapshotPolicyDetails)
		}
	}

	snapshotPolicy := &types.SnapshotPolicy{}
	Debug = true
	ifDebugLogPayload(snapshotPolicyParam)
	URL := c.urlPrefix() + Replication + SymmetrixX + symID + SnapshotPolicy
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Post(ctx, URL, c.getDefaultHeaders(), snapshotPolicyParam, snapshotPolicy)
	if err != nil {
		log.Error("Create Snapshot Policy failed: " + err.Error())
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully created Snapshot Policy: %s", snapshotPolicyID))
	return snapshotPolicy, nil
}

// UpdateSnapshotPolicy is a general method to update a SnapshotPolicy (PUT operation) based on the action using a UpdateSnapshotPolicyPayload.
func (c *Client) UpdateSnapshotPolicy(ctx context.Context, symID string, action string, snapshotPolicyID string, optionalPayload map[string]interface{}) error {
	defer c.TimeSpent("UpdateSnapshotPolicy", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}

	updateSnapshotPolicyParam := &types.UpdateSnapshotPolicyParam{
		Action:          action,
		ExecutionOption: types.ExecutionOptionSynchronous,
	}

	if len(optionalPayload) > 0 {
		modifySnapshotPolicyParam, okModify := optionalPayload["modify"]
		if okModify {
			updateSnapshotPolicyParam.ModifySnapshotPolicyParam = modifySnapshotPolicyParam.(*types.ModifySnapshotPolicyParam)
		}
		associateStorageGroupParam, okAssociate := optionalPayload["associateStorageGroupParam"]
		if okAssociate {
			updateSnapshotPolicyParam.AssociateStorageGroupParam = associateStorageGroupParam.(*types.AssociateStorageGroupParam)
		}
		disassociateStorageGroupParam, okDisassociate := optionalPayload["disassociateStorageGroupParam"]
		if okDisassociate {
			updateSnapshotPolicyParam.DisassociateStorageGroupParam = disassociateStorageGroupParam.(*types.DisassociateStorageGroupParam)
		}
	}

	URL := c.urlPrefix() + Replication + SymmetrixX + symID + SnapshotPolicy + "/" + snapshotPolicyID
	fields := map[string]interface{}{
		http.MethodPut: URL,
	}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), updateSnapshotPolicyParam, nil)
	if err != nil {
		log.WithFields(fields).Error("Error in UpdateSnapshotPolicy: " + err.Error())
		return err
	}
	return nil
}

// GetSnapshotPolicyList returns all the SnapshotPolicy names given the Symmetrix ID
func (c *Client) GetSnapshotPolicyList(ctx context.Context, symID string) (*types.SnapshotPolicyList, error) {
	defer c.TimeSpent("GetSnapshotPolicyList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + Replication + SymmetrixX + symID + SnapshotPolicy
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetSnapshotPolicyList failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	snapshotPolicies := &types.SnapshotPolicyList{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(snapshotPolicies); err != nil {
		return nil, err
	}

	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return snapshotPolicies, nil
}
