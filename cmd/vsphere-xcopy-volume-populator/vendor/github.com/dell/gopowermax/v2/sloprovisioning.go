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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	types "github.com/dell/gopowermax/v2/types/v100"
	log "github.com/sirupsen/logrus"
)

// The follow constants are for internal use within the pmax library.
const (
	SLOProvisioningX       = "sloprovisioning/"
	SnapshotPolicy         = "/snapshot_policy"
	SymmetrixX             = "symmetrix/"
	IteratorX              = "common/Iterator/"
	XPage                  = "/page"
	XVolume                = "/volume"
	XStorageGroup          = "/storagegroup"
	XPortGroup             = "/portgroup"
	XInitiator             = "/initiator"
	XHost                  = "/host"
	XHostGroup             = "/hostgroup"
	XMaskingView           = "/maskingview"
	Emulation              = "FBA"
	MaxVolIdentifierLength = 64
	Migration              = "migration/"
)

// TimeSpent - Calculates and prints time spent for a caller function
func (c *Client) TimeSpent(functionName string, startTime time.Time) {
	if logResponseTimes {
		if functionName == "" {
			pc, _, _, ok := runtime.Caller(1)
			details := runtime.FuncForPC(pc)
			if ok && details != nil {
				functionName = details.Name()
			}
		}
		endTime := time.Now()
		log.Infof("pmax-time: %s took %.2f seconds to complete", functionName, endTime.Sub(startTime).Seconds())
	}
}

// GetVolumeIDsIterator returns a VolumeIDs Iterator. It generally fetches the first page in the result as part of the operation.
func (c *Client) GetVolumeIDsIterator(ctx context.Context, symID string, volumeIdentifierMatch string, like bool) (*types.VolumeIterator, error) {
	defer c.TimeSpent("GetVolumeIDsIterator", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	var query string
	if volumeIdentifierMatch != "" {
		if like {
			query = fmt.Sprintf("?volume_identifier=%%3Clike%%3E%s", volumeIdentifierMatch)
		} else {
			query = fmt.Sprintf("?volume_identifier=%s", volumeIdentifierMatch)
		}
	}

	return c.getVolumeIDsIteratorBase(ctx, symID, query)
}

// GetVolumesInStorageGroupIterator returns a iterator of a list of volumes associated with a StorageGroup.
func (c *Client) GetVolumesInStorageGroupIterator(ctx context.Context, symID string, storageGroupID string) (*types.VolumeIterator, error) {
	var query string
	if storageGroupID == "" {
		return nil, fmt.Errorf("storageGroupID is empty")
	}

	query = fmt.Sprintf("?storageGroupId=%s", storageGroupID)
	return c.getVolumeIDsIteratorBase(ctx, symID, query)
}

// GetVolumeIDsIteratorWithParams returns an iterator of a list of volumes with query parameters
// For multiple parameters in single field, use ',' to separate the values
func (c *Client) GetVolumeIDsIteratorWithParams(ctx context.Context, symID string, queryParams map[string]string) (*types.VolumeIterator, error) {
	defer c.TimeSpent("GetVolumeIDsIteratorWithParams", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	query := ""
	if queryParams != nil {
		query += "?"
		for key, val := range queryParams {
			for _, subVal := range strings.Split(val, ",") {
				// if value starts with > or <, directly add it
				if regexp.MustCompile("^[><]\\d+(\\.\\d+)?$").MatchString(subVal) {
					query += fmt.Sprintf("%s%s", key, val)
				} else {
					// remove the first '='
					if subVal[0] == '=' {
						subVal = subVal[1:]
					}
					query += fmt.Sprintf("%s=%s", key, subVal)
				}
				query += "&"
			}
		}
		query = query[:len(query)-1]
	}
	return c.getVolumeIDsIteratorBase(ctx, symID, query)
}

// GetVolumeIDsIterator returns a VolumeIDs Iterator. It generally fetches the first page in the result as part of the operation.
func (c *Client) getVolumeIDsIteratorBase(ctx context.Context, symID string, query string) (*types.VolumeIterator, error) {
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XVolume
	if query != "" {
		URL = URL + query
	}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetVolumeIDList failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	iter := &types.VolumeIterator{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(iter); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// GetVolumeIDsIteratorPage fetches the next page of the iterator's result. From is the starting point. To can be left as 0, or can be set to the last element desired.
func (c *Client) GetVolumeIDsIteratorPage(ctx context.Context, iter *types.VolumeIterator, from, to int) ([]string, error) {
	defer c.TimeSpent("GetVolumeIDsIteratorPage", time.Now())
	if to == 0 || to-from+1 > iter.MaxPageSize {
		to = from + iter.MaxPageSize - 1
	}
	if to > iter.Count {
		to = iter.Count
	}
	queryParams := fmt.Sprintf("?from=%d&to=%d", from, to)
	URL := RESTPrefix + IteratorX + iter.ID + XPage + queryParams

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetVolumeIDsIteratorPage failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	result := &types.VolumeResultList{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(result); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	volumeIDList := make([]string, to-from+1)
	for i := range result.VolumeList {
		volumeIDList[i] = result.VolumeList[i].VolumeIDs
	}
	return volumeIDList, nil
}

// DeleteVolumeIDsIterator deletes a volume iterator.
func (c *Client) DeleteVolumeIDsIterator(ctx context.Context, iter *types.VolumeIterator) error {
	defer c.TimeSpent("DeleteVolumeIDsIterator", time.Now())
	URL := RESTPrefix + IteratorX + iter.ID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Delete(ctx, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		return err
	}
	return nil
}

// GetVolumeIDList gets a list of matching volume ids. If volumeIdentifierMatch is the empty string,
// all volumes are returned. Otherwise the volumes are filtered to volumes whose VolumeIdentifier
// exactly matches the volumeIdentfierMatch argument (when like is false), or whose VolumeIdentifier
// contains the volumeIdentifierMatch argument (when like is true).
func (c *Client) GetVolumeIDList(ctx context.Context, symID string, volumeIdentifierMatch string, like bool) ([]string, error) {
	defer c.TimeSpent("GetVolumeIDList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	iter, err := c.GetVolumeIDsIterator(ctx, symID, volumeIdentifierMatch, like)
	if err != nil {
		return nil, err
	}
	return c.volumeIteratorToVolIDList(ctx, iter)
}

// GetVolumeIDListInStorageGroup - Gets a list of volume in a SG
func (c *Client) GetVolumeIDListInStorageGroup(ctx context.Context, symID string, storageGroupID string) ([]string, error) {
	iter, err := c.GetVolumesInStorageGroupIterator(ctx, symID, storageGroupID)
	if err != nil {
		return nil, err
	}
	return c.volumeIteratorToVolIDList(ctx, iter)
}

// GetVolumeIDListWithParams - Gets a list of volume ids with parameters
func (c *Client) GetVolumeIDListWithParams(ctx context.Context, symID string, queryParams map[string]string) ([]string, error) {
	defer c.TimeSpent("GetVolumeIDListWithParams", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	iter, err := c.GetVolumeIDsIteratorWithParams(ctx, symID, queryParams)
	if err != nil {
		return nil, err
	}
	return c.volumeIteratorToVolIDList(ctx, iter)
}

func (c *Client) volumeIteratorToVolIDList(ctx context.Context, iter *types.VolumeIterator) ([]string, error) {
	if iter.MaxPageSize < iter.Count {
		// The iterator only needs to be deleted if there are more entries than MaxPageSize?
		defer func(c *Client, ctx context.Context, iter *types.VolumeIterator) {
			err := c.DeleteVolumeIDsIterator(ctx, iter)
			if err != nil {
				return
			}
		}(c, ctx, iter)
	}

	// Get the initial results
	result := iter.ResultList
	volumeIDList := make([]string, len(result.VolumeList))
	for i := range result.VolumeList {
		volumeIDList[i] = result.VolumeList[i].VolumeIDs
	}

	// Iterate through additional pages
	for from := result.To + 1; from <= iter.Count; {
		idlist, err := c.GetVolumeIDsIteratorPage(ctx, iter, from, 0)
		if err != nil {
			return nil, err
		}
		volumeIDList = append(volumeIDList, idlist...)
		from = from + len(idlist)
	}
	if len(volumeIDList) != iter.Count {
		return nil, fmt.Errorf("Expected %d ids but got %d ids", iter.Count, len(volumeIDList))
	}
	return volumeIDList, nil
}

// GetVolumeByID returns a Volume structure given the symmetrix and volume ID (volume ID is 5-digit hex field)
func (c *Client) GetVolumeByID(ctx context.Context, symID string, volumeID string) (*types.Volume, error) {
	defer c.TimeSpent("GetVolumeByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XVolume + "/" + volumeID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetVolumeByID failed: " + err.Error())
		return nil, err
	}
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	volume := &types.Volume{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(volume); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return volume, nil
}

// GetStorageGroupIDList returns a list of StorageGroupIds in a StorageGroupIDList type.
func (c *Client) GetStorageGroupIDList(ctx context.Context, symID, storageGroupIDMatch string, like bool) (*types.StorageGroupIDList, error) {
	defer c.TimeSpent("GetStorageGroupIDList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	var query string
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XStorageGroup
	if storageGroupIDMatch != "" {
		if like {
			query = fmt.Sprintf("?storageGroupId=%%3Clike%%3E%s", storageGroupIDMatch)
		} else {
			query = fmt.Sprintf("?storageGroupId=%s", storageGroupIDMatch)
		}
		URL = fmt.Sprintf("%s%s", URL, query)
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetStorageGroupIDList failed: " + err.Error())
		return nil, err
	}
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	sgIDList := &types.StorageGroupIDList{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(sgIDList); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return sgIDList, nil
}

// GetCreateStorageGroupPayload returns U4P payload for creating storage group
func (c *Client) GetCreateStorageGroupPayload(storageGroupID, srpID, serviceLevel string, thickVolumes bool, optionalPayload map[string]interface{}) (payload interface{}) {
	workload := "None"
	sloParams := []types.SLOBasedStorageGroupParam{}
	var snapshotPolicies []string
	if srpID != "None" {
		sloParams = []types.SLOBasedStorageGroupParam{
			{
				SLOID:             serviceLevel,
				WorkloadSelection: workload,
				VolumeAttributes: []types.VolumeAttributeType{
					{
						VolumeSize:      "0",
						CapacityUnit:    "CYL",
						NumberOfVolumes: 0,
					},
				},
				AllocateCapacityForEachVol: thickVolumes,
				// compression not allowed with thick volumes
				NoCompression: thickVolumes,
			},
		}

		if len(optionalPayload) > 0 {
			hostLimit, ok := optionalPayload["hostLimits"]
			if ok {
				sloParams[0].SetHostIOLimitsParam = hostLimit.(*types.SetHostIOLimitsParam)
			}
			snapshotPolicies, _ = optionalPayload["snapshotPolicies"].([]string)
		}
	}
	createStorageGroupParam := &types.CreateStorageGroupParam{
		StorageGroupID:            storageGroupID,
		SRPID:                     srpID,
		Emulation:                 Emulation,
		ExecutionOption:           types.ExecutionOptionSynchronous,
		SLOBasedStorageGroupParam: sloParams,
		SnapshotPolicies:          snapshotPolicies,
	}
	return createStorageGroupParam
}

// CreateStorageGroup creates a Storage Group given the storageGroupID (name), srpID (storage resource pool), service level, and boolean for thick volumes.
// If srpID is "None" then serviceLevel and thickVolumes settings are ignored
func (c *Client) CreateStorageGroup(ctx context.Context, symID, storageGroupID, srpID, serviceLevel string, thickVolumes bool, optionalPayload map[string]interface{}) (*types.StorageGroup, error) {
	defer c.TimeSpent("CreateStorageGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XStorageGroup
	payload := c.GetCreateStorageGroupPayload(storageGroupID, srpID, serviceLevel, thickVolumes, optionalPayload)
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodPost, URL, c.getDefaultHeaders(), payload)
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	storageGroup := &types.StorageGroup{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(storageGroup); err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully created SG: %s", storageGroupID))
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return storageGroup, nil
}

// DeleteStorageGroup deletes a storage group
func (c *Client) DeleteStorageGroup(ctx context.Context, symID string, storageGroupID string) error {
	defer c.TimeSpent("DeleteStorageGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XStorageGroup + "/" + storageGroupID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Delete(ctx, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("DeleteStorageGroup failed: " + err.Error())
		return err
	}
	log.Info(fmt.Sprintf("Successfully deleted SG: %s", storageGroupID))
	return nil
}

// DeleteMaskingView deletes a storage group
func (c *Client) DeleteMaskingView(ctx context.Context, symID string, maskingViewID string) error {
	defer c.TimeSpent("DeleteMaskingView", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XMaskingView + "/" + maskingViewID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Delete(ctx, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("DeleteMaskingView failed: " + err.Error())
		return err
	}
	log.Info(fmt.Sprintf("Successfully deleted Masking View: %s", maskingViewID))
	return nil
}

// GetStorageGroup returns a StorageGroup given the Symmetrix ID and Storage Group ID (which is really a name).
func (c *Client) GetStorageGroup(ctx context.Context, symID string, storageGroupID string) (*types.StorageGroup, error) {
	defer c.TimeSpent("GetStorageGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XStorageGroup + "/" + storageGroupID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetStorageGroup failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	storageGroup := &types.StorageGroup{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(storageGroup); err != nil {
		return nil, err
	}

	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return storageGroup, nil
}

// GetStorageGroupSnapshotPolicy returns a StorageGroup snapshotPolicy given the Symmetrix ID, Storage Group ID (which is really a name) and Snapshot Policy ID (which is really a name).
func (c *Client) GetStorageGroupSnapshotPolicy(ctx context.Context, symID, snapshotPolicyID, storageGroupID string) (*types.StorageGroupSnapshotPolicy, error) {
	defer c.TimeSpent("GetStorageGroupSnapshotPolicy", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	URL := c.urlPrefix() + Replication + SymmetrixX + symID + SnapshotPolicy + "/" + snapshotPolicyID + XStorageGroup + "/" + storageGroupID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()

	storageGroupSnapshotPolicy := &types.StorageGroupSnapshotPolicy{}
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), storageGroupSnapshotPolicy)
	if err != nil {
		log.Error("GetStorageGroupSnapshotPolicy failed: " + err.Error())
		return nil, err
	}

	return storageGroupSnapshotPolicy, nil
}

// GetStoragePool returns a StoragePool given the Symmetrix ID and Storage Pool ID
func (c *Client) GetStoragePool(ctx context.Context, symID string, storagePoolID string) (*types.StoragePool, error) {
	defer c.TimeSpent("GetStoragePool", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + "/" + StorageResourcePool + "/" + storagePoolID
	storagePool := &types.StoragePool{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), storagePool)
	if err != nil {
		log.Error("GetStoragePool failed: " + err.Error())
		return nil, err
	}
	return storagePool, nil
}

// UpdateStorageGroup is a general method to update a StorageGroup (PUT operation) using a UpdateStorageGroupPayload.
func (c *Client) UpdateStorageGroup(ctx context.Context, symID string, storageGroupID string, payload interface{}) (*types.Job, error) {
	defer c.TimeSpent("UpdateStorageGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XStorageGroup + "/" + storageGroupID
	job := &types.Job{}
	fields := map[string]interface{}{
		http.MethodPut: URL,
	}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), payload, job)
	if err != nil {
		log.WithFields(fields).Error("Error in UpdateStorageGroup: " + err.Error())
		return nil, err
	}
	return job, nil
}

// UpdateStorageGroupS is a general method to update a StorageGroup (PUT operation) using a UpdateStorageGroupPayload.
func (c *Client) UpdateStorageGroupS(ctx context.Context, symID string, storageGroupID string, payload interface{}) error {
	defer c.TimeSpent("UpdateStorageGroupS", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XStorageGroup + "/" + storageGroupID
	fields := map[string]interface{}{
		http.MethodPut: URL,
	}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), payload, nil)
	if err != nil {
		log.WithFields(fields).Error("Error in UpdateStorageGroup: " + err.Error())
		return err
	}
	return nil
}

func ifDebugLogPayload(payload interface{}) {
	if Debug == false {
		return
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Error("could not Marshal json payload: " + err.Error())
	} else {
		log.Info("payload: " + string(payloadBytes))
	}
}

// CreateVolumeInStorageGroup creates a volume in the specified Storage Group with a given volumeName
// and the size of the volume in cylinders.
func (c *Client) CreateVolumeInStorageGroup(ctx context.Context, symID string, storageGroupID string, volumeName string, volumeSize interface{}, volOpts map[string]interface{}) (*types.Volume, error) {
	capUnit := "CYL"
	enableMobility := false
	defer c.TimeSpent("CreateVolumeInStorageGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	if len(volOpts) > 0 {
		if value, ok := volOpts["capacityUnit"]; ok {
			if val, isUnit := value.(string); isUnit {
				capUnit = val
			} else {
				return nil, fmt.Errorf("invalid capacityUnit for creation of volume")
			}
		}
		if value, ok := volOpts["enableMobility"]; ok {
			if mobility, isBool := value.(bool); isBool {
				enableMobility = mobility
			}
		}
	}

	if len(volumeName) > MaxVolIdentifierLength {
		return nil, fmt.Errorf("Length of volumeName exceeds max limit")
	}

	job := &types.Job{}
	var err error
	payload := c.GetCreateVolInSGPayload(volumeSize, capUnit, volumeName, false, enableMobility, "", "")
	job, err = c.UpdateStorageGroup(ctx, symID, storageGroupID, payload)
	if err != nil || job == nil {
		return nil, fmt.Errorf("A job was not returned from UpdateStorageGroup")
	}
	job, err = c.WaitOnJobCompletion(ctx, symID, job.JobID)
	if err != nil {
		return nil, err
	}

	switch job.Status {
	case types.JobStatusFailed:
		return nil, fmt.Errorf("the UpdateStorageGroup job failed: %s", c.JobToString(job))
	}
	volume, err := c.GetVolumeByIdentifier(ctx, symID, storageGroupID, volumeName, volumeSize, capUnit)
	return volume, err
}

// GetVolumeByIdentifier on the given symmetrix in specific storage group with a volume name and having size in cylinders
func (c *Client) GetVolumeByIdentifier(ctx context.Context, symID, storageGroupID string, volumeName string, volumeSize interface{}, capUnit string) (*types.Volume, error) {
	var volSizeInCyl int
	var volSizeInBytes float64
	var err error
	if valueInInt, isInt := volumeSize.(int); isInt {
		volSizeInCyl = valueInInt
	} else if valueInString, isString := volumeSize.(string); isString {
		volSizeInBytes, err = strconv.ParseFloat(valueInString, 64)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse volume size to uniquely identify the volume")
		}
	}
	volIDList, err := c.GetVolumeIDList(ctx, symID, volumeName, false)
	if err != nil {
		return nil, fmt.Errorf("couldn't get Volume ID List: %s", err.Error())
	}
	if len(volIDList) > 1 {
		log.Warning("Found multiple volumes matching the identifier " + volumeName)
	}
	for _, volumeID := range volIDList {
		vol, err := c.GetVolumeByID(ctx, symID, volumeID)
		if err == nil {
			for _, sgID := range vol.StorageGroupIDList {
				if sgID == storageGroupID {
					if (capUnit == "CYL" && vol.CapacityCYL == volSizeInCyl) || (capUnit == "GB" && vol.CapacityGB == volSizeInBytes) || (capUnit == "MB" && vol.FloatCapacityMB == volSizeInBytes+1) || (capUnit == "TB" && vol.CapacityGB == volSizeInBytes*1024) {
						// Return the first match
						return vol, nil
					}
				}
			}
		}
	}
	err = fmt.Errorf("failed to find newly created volume with name: %s in SG: %s", volumeName, storageGroupID)
	log.Error(err)
	return nil, err
}

// CreateVolumeInStorageGroupS creates a volume in the specified Storage Group with a given volumeName
// and the size of the volume in cylinders.
// This method is run synchronously
func (c *Client) CreateVolumeInStorageGroupS(ctx context.Context, symID, storageGroupID string, volumeName string, volumeSize interface{}, volOpts map[string]interface{}, opts ...http.Header) (*types.Volume, error) {
	defer c.TimeSpent("CreateVolumeInStorageGroup", time.Now())
	capUnit := "CYL"
	enableMobility := false

	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	if len(volOpts) > 0 {
		if value, ok := volOpts["capacityUnit"]; ok {
			if val, isUnit := value.(string); isUnit {
				capUnit = val
			} else {
				return nil, fmt.Errorf("invalid capacityUnit for creation of volume")
			}
		}
		if value, ok := volOpts["enableMobility"]; ok {
			if mobility, isBool := value.(bool); isBool {
				enableMobility = mobility
			}
		}
	}

	if len(volumeName) > MaxVolIdentifierLength {
		return nil, fmt.Errorf("Length of volumeName exceeds max limit")
	}

	payload := c.GetCreateVolInSGPayload(volumeSize, capUnit, volumeName, true, enableMobility, "", "", opts...)
	err := c.UpdateStorageGroupS(ctx, symID, storageGroupID, payload)
	if err != nil {
		return nil, fmt.Errorf("couldn't create volume. error - %s", err.Error())
	}

	volume, err := c.GetVolumeByIdentifier(ctx, symID, storageGroupID, volumeName, volumeSize, capUnit)
	return volume, err
}

// CreateVolumeInProtectedStorageGroupS takes simplified input arguments to create a volume of a give name and size in a protected storage group.
// This will add volume in both Local and Remote Storage group
// This method is run synchronously
func (c *Client) CreateVolumeInProtectedStorageGroupS(ctx context.Context, symID, remoteSymID, storageGroupID string, remoteStorageGroupID string, volumeName string, volumeSize interface{}, volOpts map[string]interface{}, opts ...http.Header) (*types.Volume, error) {
	defer c.TimeSpent("CreateVolumeInStorageGroup", time.Now())
	capUnit := "CYL"
	enableMobility := false

	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	if len(volOpts) > 0 {
		if value, ok := volOpts["capacityUnit"]; ok {
			if val, isUnit := value.(string); isUnit {
				capUnit = val
			} else {
				return nil, fmt.Errorf("invalid capacityUnit for creation of volume")
			}
		}
		if value, ok := volOpts["enableMobility"]; ok {
			if mobility, isBool := value.(bool); isBool {
				enableMobility = mobility
			}
		}
	}

	if len(volumeName) > MaxVolIdentifierLength {
		return nil, fmt.Errorf("Length of volumeName exceeds max limit")
	}

	payload := c.GetCreateVolInSGPayload(volumeSize, capUnit, volumeName, true, enableMobility, remoteSymID, remoteStorageGroupID, opts...)
	err := c.UpdateStorageGroupS(ctx, symID, storageGroupID, payload)
	if err != nil {
		return nil, fmt.Errorf("couldn't create volume. error - %s", err.Error())
	}

	volume, err := c.GetVolumeByIdentifier(ctx, symID, storageGroupID, volumeName, volumeSize, capUnit)
	return volume, err
}

// ExpandVolume expands an existing volume to a new (larger) size in CYL
func (c *Client) ExpandVolume(ctx context.Context, symID string, volumeID string, rdfGNo int, volumeSize interface{}, capUnits ...string) (*types.Volume, error) {
	var size string
	capUnit := "CYL"
	if len(capUnits) > 0 {
		capUnit = capUnits[0]
	}
	if val, isInt := volumeSize.(int); isInt {
		size = strconv.Itoa(val)
	} else if val, isString := volumeSize.(string); isString {
		size = val
	}
	payload := &types.EditVolumeParam{
		EditVolumeActionParam: types.EditVolumeActionParam{
			ExpandVolumeParam: &types.ExpandVolumeParam{
				VolumeAttribute: types.VolumeAttributeType{
					VolumeSize:   size,
					CapacityUnit: capUnit,
				},
			},
		},
	}
	if rdfGNo > 0 {
		// This expands is for replicated volume
		payload.EditVolumeActionParam.ExpandVolumeParam.RDFGroupNumber = rdfGNo
	}

	payload.ExecutionOption = types.ExecutionOptionSynchronous
	ifDebugLogPayload(payload)
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XVolume + "/" + volumeID
	err := c.api.Put(ctx, URL, c.getDefaultHeaders(), payload, nil)

	var vol *types.Volume
	if err == nil {
		vol, err = c.GetVolumeByID(ctx, symID, volumeID)
	}

	return vol, err
}

// AddVolumesToStorageGroup adds one or more volumes (given by their volumeIDs) to a StorageGroup.
func (c *Client) AddVolumesToStorageGroup(ctx context.Context, symID, storageGroupID string, force bool, volumeIDs ...string) error {
	defer c.TimeSpent("AddVolumesToStorageGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	// Check if the volume id list is not empty
	if len(volumeIDs) == 0 {
		return fmt.Errorf("At least one volume id has to be specified")
	}
	payload := c.GetAddVolumeToSGPayload(false, force, "", "", volumeIDs...)
	job, err := c.UpdateStorageGroup(ctx, symID, storageGroupID, payload)
	if err != nil || job == nil {
		return fmt.Errorf("A job was not returned from UpdateStorageGroup")
	}
	job, err = c.WaitOnJobCompletion(ctx, symID, job.JobID)
	if err != nil {
		return err
	}

	switch job.Status {
	case types.JobStatusFailed:
		return fmt.Errorf("error: UpdateStorageGroup job failed: %s", c.JobToString(job))
	}
	return nil
}

// AddVolumesToStorageGroupS adds one or more volumes (given by their volumeIDs) to a StorageGroup.
func (c *Client) AddVolumesToStorageGroupS(ctx context.Context, symID, storageGroupID string, force bool, volumeIDs ...string) error {
	defer c.TimeSpent("AddVolumesToStorageGroupS", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	// Check if the volume id list is not empty
	if len(volumeIDs) == 0 {
		return fmt.Errorf("at least one volume id has to be specified")
	}
	payload := c.GetAddVolumeToSGPayload(true, force, "", "", volumeIDs...)
	err := c.UpdateStorageGroupS(ctx, symID, storageGroupID, payload)
	if err != nil {
		return fmt.Errorf("An error(%s) was returned from UpdateStorageGroup", err.Error())
	}
	return nil
}

// AddVolumesToProtectedStorageGroup adds one or more volumes (given by their volumeIDs) to a Protected StorageGroup.
func (c *Client) AddVolumesToProtectedStorageGroup(ctx context.Context, symID, storageGroupID, remoteSymID, remoteStorageGroupID string, force bool, volumeIDs ...string) error {
	defer c.TimeSpent("AddVolumesToProtectedStorageGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	// Check if the volume id list is not empty
	if len(volumeIDs) == 0 {
		return fmt.Errorf("at least one volume id has to be specified")
	}
	payload := c.GetAddVolumeToSGPayload(true, force, remoteSymID, remoteStorageGroupID, volumeIDs...)
	err := c.UpdateStorageGroupS(ctx, symID, storageGroupID, payload)
	if err != nil {
		return fmt.Errorf("An error(%s) was returned from UpdateStorageGroup", err.Error())
	}
	return nil
}

// RemoveVolumesFromStorageGroup removes one or more volumes (given by their volumeIDs) from a StorageGroup.
func (c *Client) RemoveVolumesFromStorageGroup(ctx context.Context, symID string, storageGroupID string, force bool, volumeIDs ...string) (*types.StorageGroup, error) {
	defer c.TimeSpent("RemoveVolumesFromStorageGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	// Check if the volume id list is not empty
	if len(volumeIDs) == 0 {
		return nil, fmt.Errorf("at least one volume id has to be specified")
	}
	payload := c.GetRemoveVolumeFromSGPayload(force, "", "", volumeIDs...)
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XStorageGroup + "/" + storageGroupID
	fields := map[string]interface{}{
		http.MethodPut: URL,
	}

	updatedStorageGroup := &types.StorageGroup{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), payload, updatedStorageGroup)
	if err != nil {
		log.WithFields(fields).Error("Error in RemoveVolumesFromStorageGroup: " + err.Error())
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully removed volumes: [%s] from SG: %s", strings.Join(volumeIDs, " "), storageGroupID))
	return updatedStorageGroup, nil
}

// RemoveVolumesFromProtectedStorageGroup removes one or more volumes (given by their volumeIDs) from a Protected StorageGroup.
func (c *Client) RemoveVolumesFromProtectedStorageGroup(ctx context.Context, symID string, storageGroupID, remoteSymID, remoteStorageGroupID string, force bool, volumeIDs ...string) (*types.StorageGroup, error) {
	defer c.TimeSpent("RemoveVolumesFromStorageGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	// Check if the volume id list is not empty
	if len(volumeIDs) == 0 {
		return nil, fmt.Errorf("at least one volume id has to be specified")
	}
	payload := c.GetRemoveVolumeFromSGPayload(force, remoteSymID, remoteStorageGroupID, volumeIDs...)
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XStorageGroup + "/" + storageGroupID
	fields := map[string]interface{}{
		http.MethodPut: URL,
	}

	updatedStorageGroup := &types.StorageGroup{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), payload, updatedStorageGroup)
	if err != nil {
		log.WithFields(fields).Error("Error in RemoveVolumesFromProtectedStorageGroup: " + err.Error())
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully removed volumes: [%s] from SG: %s", strings.Join(volumeIDs, " "), storageGroupID))
	return updatedStorageGroup, nil
}

// GetCreateVolInSGPayload returns payload for adding volume/s to SG.
// if remoteSymID is passed then the payload includes RemoteSymmSGInfoParam.
func (c *Client) GetCreateVolInSGPayload(volumeSize interface{}, capUnit string, volumeName string, isSync, enableMobility bool, remoteSymID, remoteStorageGroupID string, opts ...http.Header) (payload interface{}) {
	var executionOption, size string
	if val, isInt := volumeSize.(int); isInt {
		size = strconv.Itoa(val)
	} else if val, isString := volumeSize.(string); isString {
		size = val
	}
	if isSync {
		executionOption = types.ExecutionOptionSynchronous
	} else {
		executionOption = types.ExecutionOptionAsynchronous
	}
	addVolumeParam := &types.AddVolumeParam{
		CreateNewVolumes: true,
		Emulation:        "FBA",
		EnableMobilityID: enableMobility,
		VolumeAttributes: []types.VolumeAttributeType{
			{
				NumberOfVolumes: 1,
				VolumeIdentifier: &types.VolumeIdentifierType{
					VolumeIdentifierChoice: "identifier_name",
					IdentifierName:         volumeName,
				},
				CapacityUnit: capUnit,
				VolumeSize:   size,
			},
		},
		RemoteSymmetrixSGInfo: types.RemoteSymmSGInfoParam{
			Force: true,
		},
	}
	if remoteSymID != "" {
		addVolumeParam.RemoteSymmetrixSGInfo.RemoteSymmetrix1ID = remoteSymID
		addVolumeParam.RemoteSymmetrixSGInfo.RemoteSymmetrix1SGs = []string{remoteStorageGroupID}
	}
	payload = &types.UpdateStorageGroupPayload{
		EditStorageGroupActionParam: types.EditStorageGroupActionParam{
			ExpandStorageGroupParam: &types.ExpandStorageGroupParam{
				AddVolumeParam: addVolumeParam,
			},
		},
		ExecutionOption: executionOption,
	}
	if opts != nil && len(opts) != 0 {
		// If the payload has a SetMetaData method, set the metadata headers.
		if t, ok := interface{}(payload).(interface {
			SetMetaData(metadata http.Header)
		}); ok {
			t.SetMetaData(opts[0])
		} else {
			log.Println("warning: gopowermax.UpdateStorageGroupPayload: no SetMetaData method exists, consider updating gopowermax library.")
		}
	}
	if payload != nil {
		ifDebugLogPayload(payload)
	}
	return payload
}

// GetAddVolumeToSGPayload returns payload for adding specific volume/s to SG.
func (c *Client) GetAddVolumeToSGPayload(isSync, force bool, remoteSymID, remoteStorageGroupID string, volumeIDs ...string) (payload interface{}) {
	executionOption := ""
	if isSync {
		executionOption = types.ExecutionOptionSynchronous
	} else {
		executionOption = types.ExecutionOptionAsynchronous
	}
	addSpecificVolumeParam := &types.AddSpecificVolumeParam{
		VolumeIDs: volumeIDs,
		RemoteSymmetrixSGInfo: types.RemoteSymmSGInfoParam{
			Force: force,
		},
	}
	if remoteSymID != "" {
		addSpecificVolumeParam.RemoteSymmetrixSGInfo.RemoteSymmetrix1ID = remoteSymID
		addSpecificVolumeParam.RemoteSymmetrixSGInfo.RemoteSymmetrix1SGs = []string{remoteStorageGroupID}
	}
	payload = &types.UpdateStorageGroupPayload{
		EditStorageGroupActionParam: types.EditStorageGroupActionParam{
			ExpandStorageGroupParam: &types.ExpandStorageGroupParam{
				AddSpecificVolumeParam: addSpecificVolumeParam,
			},
		},
		ExecutionOption: executionOption,
	}
	if payload != nil {
		ifDebugLogPayload(payload)
	}
	return payload
}

// GetRemoveVolumeFromSGPayload returns payload for removing volume/s from SG.
func (c *Client) GetRemoveVolumeFromSGPayload(force bool, remoteSymID, remoteStorageGroupID string, volumeIDs ...string) (payload interface{}) {
	removeVolumeParam := &types.RemoveVolumeParam{
		VolumeIDs: volumeIDs,
		RemoteSymmSGInfoParam: types.RemoteSymmSGInfoParam{
			Force: force,
		},
	}
	if remoteSymID != "" {
		removeVolumeParam.RemoteSymmSGInfoParam.RemoteSymmetrix1ID = remoteSymID
		removeVolumeParam.RemoteSymmSGInfoParam.RemoteSymmetrix1SGs = []string{remoteStorageGroupID}
	}
	payload = &types.UpdateStorageGroupPayload{
		EditStorageGroupActionParam: types.EditStorageGroupActionParam{
			RemoveVolumeParam: removeVolumeParam,
		},
		ExecutionOption: types.ExecutionOptionSynchronous,
	}
	if payload != nil {
		ifDebugLogPayload(payload)
	}
	return payload
}

// GetStoragePoolList returns a StoragePoolList object, which contains a list of all the Storage Pool names.
func (c *Client) GetStoragePoolList(ctx context.Context, symid string) (*types.StoragePoolList, error) {
	defer c.TimeSpent("GetStoragePoolList", time.Now())
	if _, err := c.IsAllowedArray(symid); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symid + "/" + StorageResourcePool
	spList := &types.StoragePoolList{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), spList)
	if err != nil {
		log.Error("GetStoragePoolList failed: " + err.Error())
		return nil, err
	}
	return spList, nil
}

// RenameVolume renames a volume.
func (c *Client) RenameVolume(ctx context.Context, symID string, volumeID string, newName string) (*types.Volume, error) {
	defer c.TimeSpent("RenameVolume", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	modifyVolumeIdentifierParam := &types.ModifyVolumeIdentifierParam{
		VolumeIdentifier: types.VolumeIdentifierType{
			VolumeIdentifierChoice: "identifier_name",
			IdentifierName:         newName,
		},
	}

	payload := &types.EditVolumeParam{
		EditVolumeActionParam: types.EditVolumeActionParam{
			ModifyVolumeIdentifierParam: modifyVolumeIdentifierParam,
		},
		ExecutionOption: types.ExecutionOptionSynchronous,
	}
	ifDebugLogPayload(payload)
	volume := &types.Volume{}

	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XVolume + "/" + volumeID
	fields := map[string]interface{}{
		http.MethodPut: URL,
		"VolumeID":     volumeID,
		"NewName":      newName,
	}
	log.WithFields(fields).Info("Renaming volume")
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), payload, volume)
	if err != nil {
		log.WithFields(fields).Error("Error in RenameVolume: " + err.Error())
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully renamed volume: %s", volumeID))
	return volume, nil
}

// DeleteVolume deletes a volume given the symmetrix ID and volume ID.
// Any storage tracks for the volume must have been previously deallocated using InitiateDeallocationOfTracksFromVolume,
// and the volume must not be a member of any Storage Group.
func (c *Client) DeleteVolume(ctx context.Context, symID string, volumeID string) error {
	defer c.TimeSpent("DeleteVolume", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XVolume + "/" + volumeID
	fields := map[string]interface{}{
		http.MethodPut: URL,
		"VolumeID":     volumeID,
	}
	log.WithFields(fields).Info("Deleting volume")
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Delete(ctx, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.WithFields(fields).Error("Error in DeleteVolume: " + err.Error())
	} else {
		log.Info(fmt.Sprintf("Successfully deleted volume: %s", volumeID))
	}
	return err
}

// InitiateDeallocationOfTracksFromVolume is an asynchrnous operation (that returns a job) to remove tracks from a volume.
func (c *Client) InitiateDeallocationOfTracksFromVolume(ctx context.Context, symID string, volumeID string) (*types.Job, error) {
	defer c.TimeSpent("InitiateDeallocationOfTracksFromVolume", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	freeVolumeParam := &types.FreeVolumeParam{
		FreeVolume: true,
	}
	payload := &types.EditVolumeParam{
		EditVolumeActionParam: types.EditVolumeActionParam{
			FreeVolumeParam: freeVolumeParam,
		},
		ExecutionOption: types.ExecutionOptionAsynchronous,
	}
	ifDebugLogPayload(payload)
	job := &types.Job{}

	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XVolume + "/" + volumeID
	fields := map[string]interface{}{
		http.MethodPut: URL,
		"VolumeID":     volumeID,
	}
	log.WithFields(fields).Info("Initiating track deletion...")
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(ctx, URL, c.getDefaultHeaders(), payload, job)
	if err != nil {
		log.WithFields(fields).Error("Error in InitiateDellocationOfTracksFromVolume: " + err.Error())
		return nil, err
	}
	return job, nil
}

// GetPortGroupList returns a PortGroupList object, which contains a list of the Port Groups
// which can be optionally filtered based on type
func (c *Client) GetPortGroupList(ctx context.Context, symID string, portGroupType string) (*types.PortGroupList, error) {
	defer c.TimeSpent("GetPortGroupList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	filter := "?"
	if strings.EqualFold(portGroupType, "fibre") {
		filter += "fibre=true"
	} else if strings.EqualFold(portGroupType, "iscsi") {
		filter += "iscsi=true"
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XPortGroup
	if len(filter) > 1 {
		URL += filter
	}
	pgList := &types.PortGroupList{}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), pgList)
	if err != nil {
		log.Error("GetPortGrouplList failed: " + err.Error())
		return nil, err
	}
	return pgList, nil
}

// GetPortGroupByID returns a PortGroup given the Symmetrix ID and Port Group ID.
func (c *Client) GetPortGroupByID(ctx context.Context, symID string, portGroupID string) (*types.PortGroup, error) {
	defer c.TimeSpent("GetPortGroupByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XPortGroup + "/" + portGroupID
	portGroup := &types.PortGroup{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), portGroup)
	if err != nil {
		log.Error("GetPortGroupByID failed: " + err.Error())
		return nil, err
	}
	return portGroup, nil
}

// GetInitiatorList returns an InitiatorList object, which contains a list of all the Initiators.
// initiatorHBA, isISCSI, inHost are optional arguments which act as filters for the initiator list
func (c *Client) GetInitiatorList(ctx context.Context, symID string, initiatorHBA string, isISCSI bool, inHost bool) (*types.InitiatorList, error) {
	defer c.TimeSpent("GetInitiatorList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	filter := "?"
	if inHost {
		filter += "in_a_host=true"
	}
	if initiatorHBA != "" {
		if len(filter) > 1 {
			filter += "&"
		}
		filter = filter + "initiator_hba=" + initiatorHBA
	}
	if isISCSI {
		if len(filter) > 1 {
			filter += "&"
		}
		filter += "iscsi=true"
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XInitiator
	if len(filter) > 1 {
		URL += filter
	}
	initList := &types.InitiatorList{}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), initList)
	if err != nil {
		log.Error("GetInitiatorList failed: " + err.Error())
		return nil, err
	}
	return initList, nil
}

// GetInitiatorByID returns an Initiator given the Symmetrix ID and Initiator ID.
func (c *Client) GetInitiatorByID(ctx context.Context, symID string, initID string) (*types.Initiator, error) {
	defer c.TimeSpent("GetInitiatorByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XInitiator + "/" + initID
	initiator := &types.Initiator{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), initiator)
	if err != nil {
		log.Error("GetInitiatorByID failed: " + err.Error())
		return nil, err
	}
	return initiator, nil
}

// GetHostList returns an HostList object, which contains a list of all the Hosts.
func (c *Client) GetHostList(ctx context.Context, symID string) (*types.HostList, error) {
	defer c.TimeSpent("GetHostList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHost
	hostList := &types.HostList{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), hostList)
	if err != nil {
		log.Error("GetHostList failed: " + err.Error())
		return nil, err
	}
	return hostList, nil
}

// GetHostByID returns a Host given the Symmetrix ID and Host ID.
func (c *Client) GetHostByID(ctx context.Context, symID string, hostID string) (*types.Host, error) {
	defer c.TimeSpent("GetHostByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHost + "/" + hostID
	host := &types.Host{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), host)
	if err != nil {
		log.Error("GetHostByID failed: " + err.Error())
		return nil, err
	}
	return host, nil
}

// CreateHost creates a host from a list of InitiatorIDs (and optional HostFlags) return returns a types.Host.
// Initiator IDs do not contain the storage port designations, just the IQN string or FC WWN.
// Initiator IDs cannot be a member of more than one host.
func (c *Client) CreateHost(ctx context.Context, symID string, hostID string, initiatorIDs []string, hostFlags *types.HostFlags) (*types.Host, error) {
	defer c.TimeSpent("CreateHost", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	hostParam := &types.CreateHostParam{
		HostID:          hostID,
		InitiatorIDs:    initiatorIDs,
		HostFlags:       hostFlags,
		ExecutionOption: types.ExecutionOptionSynchronous,
	}
	host := &types.Host{}
	Debug = true
	ifDebugLogPayload(hostParam)
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHost
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Post(ctx, URL, c.getDefaultHeaders(), hostParam, host)
	if err != nil {
		log.Error("CreateHost failed: " + err.Error())
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully created Host: %s", hostID))
	return host, nil
}

// UpdateHostFlags updates the host flags
func (c *Client) UpdateHostFlags(ctx context.Context, symID string, hostID string, hostFlags *types.HostFlags) (*types.Host, error) {
	defer c.TimeSpent("UpdateHostFlags", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	hostParam := &types.UpdateHostParam{
		EditHostAction: &types.EditHostParams{
			SetHostFlags: &types.SetHostFlags{
				HostFlags: hostFlags,
			},
		},
		ExecutionOption: types.ExecutionOptionSynchronous,
	}

	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHost + "/" + hostID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	updatedHost := &types.Host{}

	err := c.api.Put(ctx, URL, c.getDefaultHeaders(), hostParam, updatedHost)
	if err != nil {
		log.Error("UpdateHostFlags failed: " + err.Error())
		return nil, err
	}
	return updatedHost, nil
}

// UpdateHostInitiators updates a host from a list of InitiatorIDs and returns a types.Host.
func (c *Client) UpdateHostInitiators(ctx context.Context, symID string, host *types.Host, initiatorIDs []string) (*types.Host, error) {
	defer c.TimeSpent("UpdateHostInitiators", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	if host == nil {
		return nil, fmt.Errorf("Host can't be nil")
	}
	initRemove := []string{}
	initAdd := []string{}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHost + "/" + host.HostID
	updatedHost := &types.Host{}

	// figure out which initiators are being added
	for _, init := range initiatorIDs {
		// if this initiator is not in the list of current initiators, add it
		if !stringInSlice(init, host.Initiators) {
			initAdd = append(initAdd, init)
		}
	}
	// check for initiators to be removed
	for _, init := range host.Initiators {
		if !stringInSlice(init, initiatorIDs) {
			initRemove = append(initRemove, init)
		}
	}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	// add initiators if needed
	if len(initAdd) > 0 {
		hostParam := &types.UpdateHostAddInitiatorsParam{}
		hostParam.EditHostAction = &types.AddHostInitiators{}
		hostParam.EditHostAction.AddInitiator = &types.ChangeInitiatorParam{}
		hostParam.EditHostAction.AddInitiator.Initiators = initAdd
		hostParam.ExecutionOption = types.ExecutionOptionSynchronous

		ifDebugLogPayload(hostParam)
		err := c.api.Put(ctx, URL, c.getDefaultHeaders(), hostParam, updatedHost)
		if err != nil {
			log.Error("UpdateHostInitiators failed: " + err.Error())
			return nil, err
		}
	}
	// remove initiators if needed
	if len(initRemove) > 0 {
		hostParam := &types.UpdateHostRemoveInititorsParam{}
		hostParam.EditHostAction = &types.RemoveHostInitiators{}
		hostParam.EditHostAction.RemoveInitiator = &types.ChangeInitiatorParam{}
		hostParam.EditHostAction.RemoveInitiator.Initiators = initRemove
		hostParam.ExecutionOption = types.ExecutionOptionSynchronous

		ifDebugLogPayload(hostParam)
		err := c.api.Put(ctx, URL, c.getDefaultHeaders(), hostParam, updatedHost)
		if err != nil {
			log.Error("UpdateHostInitiators failed: " + err.Error())
			return nil, err
		}
	}

	return updatedHost, nil
}

// UpdateHostName updates a host with new hostID and returns a types.Host.
func (c *Client) UpdateHostName(ctx context.Context, symID, oldHostID, newHostID string) (*types.Host, error) {
	defer c.TimeSpent("UpdateHostName", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHost + "/" + oldHostID
	updatedHost := &types.Host{}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	// add initiators if needed
	if newHostID != "" {
		hostParam := &types.UpdateHostParam{}
		hostParam.EditHostAction = &types.EditHostParams{}
		hostParam.EditHostAction.RenameHostParam = &types.RenameHostParam{}
		hostParam.EditHostAction.RenameHostParam.NewHostName = newHostID
		hostParam.ExecutionOption = types.ExecutionOptionSynchronous
		ifDebugLogPayload(hostParam)
		err := c.api.Put(ctx, URL, c.getDefaultHeaders(), hostParam, updatedHost)
		if err != nil {
			log.Error("UpdateHostName failed: " + err.Error())
			return nil, err
		}
	}

	return updatedHost, nil
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// DeleteHost deletes a host entry.
func (c *Client) DeleteHost(ctx context.Context, symID string, hostID string) error {
	defer c.TimeSpent("DeleteHost", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHost + "/" + hostID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Delete(ctx, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("DeleteHost failed: " + err.Error())
		return err
	}
	log.Info(fmt.Sprintf("Successfully deleted Host: %s", hostID))
	return nil
}

// GetMaskingViewList  returns a list of the MaskingView names.
func (c *Client) GetMaskingViewList(ctx context.Context, symID string) (*types.MaskingViewList, error) {
	defer c.TimeSpent("GetMaskingViewList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XMaskingView
	mvList := &types.MaskingViewList{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), mvList)
	if err != nil {
		log.Error("GetMaskingViewList failed: " + err.Error())
		return nil, err
	}
	return mvList, nil
}

// GetMaskingViewByID returns a masking view given it's identifier (which is the name)
func (c *Client) GetMaskingViewByID(ctx context.Context, symID string, maskingViewID string) (*types.MaskingView, error) {
	defer c.TimeSpent("GetMaskingViewByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XMaskingView + "/" + maskingViewID
	mv := &types.MaskingView{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), mv)
	if err != nil {
		log.Error("GetMaskingViewByID failed: " + err.Error())
		return nil, err
	}
	return mv, nil
}

// GetMaskingViewConnections returns the connections of a masking view (optionally for a specific volume id.)
// Here volume id is the 5 digit volume ID.
func (c *Client) GetMaskingViewConnections(ctx context.Context, symID string, maskingViewID string, volumeID string) ([]*types.MaskingViewConnection, error) {
	defer c.TimeSpent("GetMaskingViewConnections", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XMaskingView + "/" + maskingViewID + "/connections"
	if volumeID != "" {
		URL = URL + "?volume_id=" + volumeID
	}
	cn := &types.MaskingViewConnectionsResult{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), cn)
	if err != nil {
		log.Error("GetMaskingViewConnections failed: " + err.Error())
		return nil, err
	}
	return cn.MaskingViewConnections, nil
}

// RenameMaskingView - Renames a masking view
func (c *Client) RenameMaskingView(ctx context.Context, symID string, maskingViewID string, newName string) (*types.MaskingView, error) {
	defer c.TimeSpent("RenameMaskingView", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	RenameMaskingViewParam := types.RenameMaskingViewParam{
		NewMaskingViewName: newName,
	}

	EditMaskingViewActionParam := types.EditMaskingViewActionParam{
		RenameMaskingViewParam: RenameMaskingViewParam,
	}

	payload := &types.EditMaskingViewParam{
		EditMaskingViewActionParam: EditMaskingViewActionParam,
		ExecutionOption:            types.ExecutionOptionSynchronous,
	}

	ifDebugLogPayload(payload)

	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XMaskingView + "/" + maskingViewID

	maskingView := &types.MaskingView{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(ctx, URL, c.getDefaultHeaders(), payload, maskingView)
	if err != nil {
		log.Error("RenameMaskingView failed: " + err.Error())
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully renamed Masking View: %s", maskingViewID))
	return maskingView, nil
}

// CreatePortGroup - Creates a Port Group
func (c *Client) CreatePortGroup(ctx context.Context, symID string, portGroupID string, dirPorts []types.PortKey, protocol string) (*types.PortGroup, error) {
	defer c.TimeSpent("CreatePortGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XPortGroup
	createPortGroupParams := &types.CreatePortGroupParams{
		PortGroupID:       portGroupID,
		SymmetrixPortKey:  dirPorts,
		ExecutionOption:   types.ExecutionOptionSynchronous,
		PortGroupProtocol: protocol,
	}
	ifDebugLogPayload(createPortGroupParams)
	portGroup := &types.PortGroup{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Post(ctx, URL, c.getDefaultHeaders(), createPortGroupParams, portGroup)
	if err != nil {
		log.Error("CreatePortGroup failed: " + err.Error())
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully created Port Group: %s", portGroupID))
	return portGroup, nil
}

// RenamePortGroup - Renames a port group
func (c *Client) RenamePortGroup(ctx context.Context, symID string, portGroupID string, newName string) (*types.PortGroup, error) {
	defer c.TimeSpent("RenamePortGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	RenamePortGroupParam := types.RenamePortGroupParam{
		NewPortGroupName: newName,
	}

	EditPortGroupActionParam := types.EditPortGroupActionParam{
		RenamePortGroupParam: &RenamePortGroupParam,
	}

	payload := &types.EditPortGroup{
		ExecutionOption:          types.ExecutionOptionSynchronous,
		EditPortGroupActionParam: &EditPortGroupActionParam,
	}

	ifDebugLogPayload(payload)

	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XPortGroup + "/" + portGroupID

	portGroup := &types.PortGroup{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(ctx, URL, c.getDefaultHeaders(), payload, portGroup)
	if err != nil {
		log.Error("RenamePortGroup failed: " + err.Error())
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully renamed Port Group: %s", portGroupID))
	return portGroup, nil
}

// CreateMaskingView creates a masking view and returns the masking view object
func (c *Client) CreateMaskingView(ctx context.Context, symID string, maskingViewID string, storageGroupID string, hostOrhostGroupID string, isHost bool, portGroupID string) (*types.MaskingView, error) {
	defer c.TimeSpent("CreateMaskingView", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XMaskingView
	useExistingStorageGroupParam := &types.UseExistingStorageGroupParam{
		StorageGroupID: storageGroupID,
	}
	useExistingPortGroupParam := &types.UseExistingPortGroupParam{
		PortGroupID: portGroupID,
	}
	hostOrHostGroupSelection := &types.HostOrHostGroupSelection{}
	if isHost {
		hostOrHostGroupSelection.UseExistingHostParam = &types.UseExistingHostParam{
			HostID: hostOrhostGroupID,
		}
	} else {
		hostOrHostGroupSelection.UseExistingHostGroupParam = &types.UseExistingHostGroupParam{
			HostGroupID: hostOrhostGroupID,
		}
	}
	createMaskingViewParam := &types.MaskingViewCreateParam{
		MaskingViewID:            maskingViewID,
		HostOrHostGroupSelection: hostOrHostGroupSelection,
		PortGroupSelection: &types.PortGroupSelection{
			UseExistingPortGroupParam: useExistingPortGroupParam,
		},
		StorageGroupSelection: &types.StorageGroupSelection{
			UseExistingStorageGroupParam: useExistingStorageGroupParam,
		},
	}
	ifDebugLogPayload(createMaskingViewParam)
	maskingView := &types.MaskingView{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Post(ctx, URL, c.getDefaultHeaders(), createMaskingViewParam, maskingView)
	if err != nil {
		log.Error("CreateMaskingView failed: " + err.Error())
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully created Masking View: %s", maskingViewID))
	return maskingView, nil
}

// DeletePortGroup - Deletes a PG
func (c *Client) DeletePortGroup(ctx context.Context, symID string, portGroupID string) error {
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XPortGroup + "/" + portGroupID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Delete(ctx, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("DeletePortGroup failed: " + err.Error())
		return err
	}
	return nil
}

// UpdatePortGroup - Update the PortGroup based on the 'ports' slice. The slice represents the intended
// configuration of the PortGroup after _successful_ completion of the request.
// NB: based on the passed in 'ports' the implementation will determine how to update
// the PortGroup and make appropriate REST calls sequentially. Take this into
// consideration when making parallel calls.
func (c *Client) UpdatePortGroup(ctx context.Context, symID string, portGroupID string, ports []types.PortKey) (*types.PortGroup, error) {
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XPortGroup + "/" + portGroupID
	fmt.Println(URL)

	// Create map of string "<DIRECTOR ID>/<PORT ID>" to a SymmetrixPortKeyType object based on the passed in 'ports'
	inPorts := make(map[string]*types.SymmetrixPortKeyType)
	for _, port := range ports {
		director := strings.ToUpper(port.DirectorID)
		port := strings.ToLower(port.PortID)
		key := fmt.Sprintf("%s/%s", director, port)
		if inPorts[key] == nil {
			inPorts[key] = &types.SymmetrixPortKeyType{
				DirectorID: director,
				PortID:     port,
			}
		}
	}

	pg, err := c.GetPortGroupByID(ctx, symID, portGroupID)
	if err != nil {
		log.Error("Could not get portGroup: " + err.Error())
		return nil, err
	}

	portIDRegex, _ := regexp.Compile("\\w+:(\\d+)")

	// Create map of string "<DIRECTOR ID>/<PORT ID>" to a SymmetrixPortKeyType object based on what's found
	// in the PortGroup
	pgPorts := make(map[string]*types.SymmetrixPortKeyType)
	for _, p := range pg.SymmetrixPortKey {
		director := strings.ToUpper(p.DirectorID)
		// PortID string may come as a combination of directory + port_number
		// Extract just the port_number part
		port := strings.ToLower(p.PortID)
		submatch := portIDRegex.FindAllStringSubmatch(port, -1)
		if len(submatch) > 0 {
			port = submatch[0][1]
		}
		key := fmt.Sprintf("%s/%s", director, port)
		pgPorts[key] = &types.SymmetrixPortKeyType{
			DirectorID: director,
			PortID:     port,
		}
	}

	// Diff ports in request with ones in PortGroup --> ports to add
	var added []types.SymmetrixPortKeyType
	for k, v := range inPorts {
		if pgPorts[k] == nil {
			added = append(added, *v)
		}
	}

	// Diff ports in PortGroup with ones in request --> ports to remove
	var removed []types.SymmetrixPortKeyType
	for k, v := range pgPorts {
		if inPorts[k] == nil {
			removed = append(removed, *v)
		}
	}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()

	if len(added) > 0 {
		log.Info(fmt.Sprintf("Adding ports %v", added))
		edit := &types.EditPortGroupActionParam{
			AddPortParam: &types.AddPortParam{
				Ports: added,
			},
		}
		add := types.EditPortGroup{
			ExecutionOption:          types.ExecutionOptionSynchronous,
			EditPortGroupActionParam: edit,
		}
		err := c.api.Put(ctx, URL, c.getDefaultHeaders(), add, &pg)
		if err != nil {
			log.Error("UpdatePortGroup failed when trying to add ports: " + err.Error())
			return nil, err
		}
	}

	if len(removed) > 0 {
		log.Info(fmt.Sprintf("Removing ports %v", removed))
		edit := &types.EditPortGroupActionParam{
			RemovePortParam: &types.RemovePortParam{
				Ports: removed,
			},
		}
		remove := types.EditPortGroup{
			ExecutionOption:          types.ExecutionOptionSynchronous,
			EditPortGroupActionParam: edit,
		}
		err := c.api.Put(ctx, URL, c.getDefaultHeaders(), remove, &pg)
		if err != nil {
			log.Error("UpdatePortGroup failed when trying to remove ports: " + err.Error())
			return nil, err
		}
	}
	return pg, nil
}

// ModifyMobilityForVolume enables/disables mobility for the volume. The volume should not be associated with any maskingview if mobility has to be enabled.
func (c *Client) ModifyMobilityForVolume(ctx context.Context, symID string, volumeID string, mobility bool) (*types.Volume, error) {
	defer c.TimeSpent("ModifyMobilityForVolume", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	EnableMobilityIDParam := &types.EnableMobilityIDParam{
		EnableMobilityID: mobility,
	}

	payload := &types.EditVolumeParam{
		EditVolumeActionParam: types.EditVolumeActionParam{
			EnableMobilityIDParam: EnableMobilityIDParam,
		},
		ExecutionOption: types.ExecutionOptionSynchronous,
	}
	ifDebugLogPayload(payload)
	volume := &types.Volume{}

	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XVolume + "/" + volumeID
	fields := map[string]interface{}{
		http.MethodPut: URL,
		"VolumeID":     volumeID,
	}
	log.WithFields(fields).Info("Modifying mobility for volume")
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), payload, volume)
	if err != nil {
		log.WithFields(fields).Error("Error in modifying mobility for volume: " + err.Error())
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully modified mobility for the volume: %s", volumeID))
	return volume, nil
}

// CreateHostGroup creates a hostGroup from a list of hostIDs (and optional HostFlags) return returns a types.HostGroup.
func (c *Client) CreateHostGroup(ctx context.Context, symID string, hostGroupID string, hostIDs []string, hostFlags *types.HostFlags) (*types.HostGroup, error) {
	defer c.TimeSpent("CreateHostGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	hostGroupParam := &types.CreateHostGroupParam{
		HostGroupID:     hostGroupID,
		HostIDs:         hostIDs,
		HostFlags:       hostFlags,
		ExecutionOption: types.ExecutionOptionSynchronous,
	}
	hostGroup := &types.HostGroup{}
	Debug = true
	ifDebugLogPayload(hostGroupParam)
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHostGroup
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Post(ctx, URL, c.getDefaultHeaders(), hostGroupParam, hostGroup)
	if err != nil {
		log.Error("CreateHostGroup failed: " + err.Error())
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully created HostGroup: %s", hostGroupID))
	return hostGroup, nil
}

// GetHostGroupByID returns a HostGroup given the Symmetrix ID and HostGroup ID.
func (c *Client) GetHostGroupByID(ctx context.Context, symID string, hostGroupID string) (*types.HostGroup, error) {
	defer c.TimeSpent("GetHostGroupByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHostGroup + "/" + hostGroupID
	hostGroup := &types.HostGroup{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), hostGroup)
	if err != nil {
		log.Error("GetHostGroupByID failed: " + err.Error())
		return nil, err
	}
	return hostGroup, nil
}

// GetHostGroupList returns an HostGroupList object, which contains a list of all the HostGroups.
func (c *Client) GetHostGroupList(ctx context.Context, symID string) (*types.HostGroupList, error) {
	defer c.TimeSpent("GetHostGroupList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHostGroup
	hostgroupList := &types.HostGroupList{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Get(ctx, URL, c.getDefaultHeaders(), hostgroupList)
	if err != nil {
		log.Error("GetHostGroupList failed: " + err.Error())
		return nil, err
	}
	return hostgroupList, nil
}

// DeleteHostGroup deletes a host entry.
func (c *Client) DeleteHostGroup(ctx context.Context, symID string, hostGroupID string) error {
	defer c.TimeSpent("DeleteHostGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHostGroup + "/" + hostGroupID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Delete(ctx, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("DeleteHostGroup failed: " + err.Error())
		return err
	}
	log.Info(fmt.Sprintf("Successfully deleted HostGroup: %s", hostGroupID))
	return nil
}

// UpdateHostGroupName updates a hostGroup with new hostGroup ID and returns a types.HostGroup.
func (c *Client) UpdateHostGroupName(ctx context.Context, symID, oldHostGroupID, newHostGroupID string) (*types.HostGroup, error) {
	defer c.TimeSpent("UpdateHostGroupName", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHostGroup + "/" + oldHostGroupID
	updatedHostGroup := &types.HostGroup{}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	if newHostGroupID != "" {
		hostGroupParam := &types.UpdateHostGroupParam{
			EditHostGroupAction: &types.EditHostGroupActionParams{
				RenameHostGroupParam: &types.RenameHostGroupParam{
					NewHostGroupName: newHostGroupID,
				},
			},
			ExecutionOption: types.ExecutionOptionSynchronous,
		}

		ifDebugLogPayload(hostGroupParam)
		err := c.api.Put(ctx, URL, c.getDefaultHeaders(), hostGroupParam, updatedHostGroup)
		if err != nil {
			log.Error("UpdateHostGroupName failed: " + err.Error())
			return nil, err
		}
	}

	return updatedHostGroup, nil
}

// UpdateHostGroupFlags updates the host flags
func (c *Client) UpdateHostGroupFlags(ctx context.Context, symID string, hostGroupID string, hostFlags *types.HostFlags) (*types.HostGroup, error) {
	defer c.TimeSpent("UpdateHostGroupFlags", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	hostParam := &types.UpdateHostGroupParam{
		EditHostGroupAction: &types.EditHostGroupActionParams{
			SetHostGroupFlags: &types.SetHostFlags{
				HostFlags: hostFlags,
			},
		},
		ExecutionOption: types.ExecutionOptionSynchronous,
	}

	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHostGroup + "/" + hostGroupID
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	updatedHostGroup := &types.HostGroup{}

	err := c.api.Put(ctx, URL, c.getDefaultHeaders(), hostParam, updatedHostGroup)
	if err != nil {
		log.Error("UpdateHostGroupFlags failed: " + err.Error())
		return nil, err
	}
	return updatedHostGroup, nil
}

// UpdateHostGroupHosts will add/remove the hosts for a host group
func (c *Client) UpdateHostGroupHosts(ctx context.Context, symID string, hostGroupID string, hostIDs []string) (*types.HostGroup, error) {
	defer c.TimeSpent("UpdateHostGroupHosts", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	if hostGroupID == "" {
		return nil, fmt.Errorf("hostgroup ID can't be empty")
	}

	hostGroup, err := c.GetHostGroupByID(ctx, symID, hostGroupID)
	if err != nil {
		return nil, err
	}

	hostRemove := []string{}
	hostAdd := []string{}
	existingHosts := []string{}
	URL := c.urlPrefix() + SLOProvisioningX + SymmetrixX + symID + XHostGroup + "/" + hostGroup.HostGroupID
	updatedHostGroup := &types.HostGroup{}

	for _, host := range hostGroup.Hosts {
		existingHosts = append(existingHosts, host.HostID)
	}

	// figure out which hosts are being added
	for _, hostID := range hostIDs {
		// if this host is not in the list of current hosts, add it
		if !stringInSlice(hostID, existingHosts) {
			hostAdd = append(hostAdd, hostID)
		}
	}

	// check for hosts to be removed
	for _, existingHost := range existingHosts {
		if !stringInSlice(existingHost, hostIDs) {
			hostRemove = append(hostRemove, existingHost)
		}
	}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()

	// add hosts if needed
	if len(hostAdd) > 0 {
		hostGroupParam := &types.UpdateHostGroupParam{
			ExecutionOption: types.ExecutionOptionSynchronous,
			EditHostGroupAction: &types.EditHostGroupActionParams{
				AddHostParam: &types.EditHostsParam{
					Host: hostAdd,
				},
			},
		}

		ifDebugLogPayload(hostGroupParam)
		err := c.api.Put(ctx, URL, c.getDefaultHeaders(), hostGroupParam, updatedHostGroup)
		if err != nil {
			log.Error("UpdateHostGroupHosts add failed: " + err.Error())
			return nil, err
		}
	}

	// remove hosts if needed
	if len(hostRemove) > 0 {
		hostGroupParam := &types.UpdateHostGroupParam{
			ExecutionOption: types.ExecutionOptionSynchronous,
			EditHostGroupAction: &types.EditHostGroupActionParams{
				RemoveHostParam: &types.EditHostsParam{
					Host: hostRemove,
				},
			},
		}

		ifDebugLogPayload(hostGroupParam)
		err := c.api.Put(ctx, URL, c.getDefaultHeaders(), hostGroupParam, updatedHostGroup)
		if err != nil {
			log.Error("UpdateHostGroupHosts remove failed: " + err.Error())
			return nil, err
		}
	}
	return updatedHostGroup, nil
}
