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
	"time"

	types "github.com/dell/gopowermax/v2/types/v100"
	log "github.com/sirupsen/logrus"
)

// The following constants are for internal use within the pmax library.
const (
	XRDFGroup      = "/rdf_group"
	XFREERDFG      = "/rdf_group_numbers_free"
	XRDFDIR        = "/rdf_director/"
	XRDFONLINEDIR  = "/rdf_director?online=true"
	XRDFPORT       = "/port/"
	XRDFPORTONLINE = "/port?online=true"
	XREMOTEPORT    = "/remote_port"
	ASYNC          = "ASYNC"
	METRO          = "METRO"
	SYNC           = "SYNC"
	XMigration     = "migration/"
	XRemoteSymID   = "?remote_symmetrix_id="
)

// GetFreeLocalAndRemoteRDFg  gets the next free RDFg available
// This API is only available in 10.x
func (c *Client) GetFreeLocalAndRemoteRDFg(ctx context.Context, localSymID string, remoteSymID string) (*types.NextFreeRDFGroup, error) {
	defer c.TimeSpent("GetFreeLocalAndRemoteRDFg", time.Now())
	if _, err := c.IsAllowedArray(localSymID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()

	// Eg: univmax/restapi/internal/100/file/symmetrix/<LocalSID>/rdf_group_numbers_free?remote_symmetrix_id=<remoteSID>
	URL := c.urlInternalPrefix() + SymmetrixX + localSymID + XFREERDFG
	if remoteSymID != "" {
		URL = fmt.Sprintf("%s?%s%s", URL, XRemoteSymID, remoteSymID)
	}
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetFreeLocalAndRemoteRDFg failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	freeRdfGroups := new(types.NextFreeRDFGroup)
	if err := json.NewDecoder(resp.Body).Decode(freeRdfGroups); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return freeRdfGroups, nil
}

// GetLocalOnlineRDFDirs gets all Online Directors for the given SYMM
func (c *Client) GetLocalOnlineRDFDirs(ctx context.Context, localSymID string) (*types.RDFDirList, error) {
	defer c.TimeSpent("GetLocalOnlineRDFDirs", time.Now())
	if _, err := c.IsAllowedArray(localSymID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()

	URL := c.urlPrefix() + ReplicationX + SymmetrixX + localSymID + XRDFONLINEDIR
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetLocalOnlineRDFDirs failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	rdfDirList := new(types.RDFDirList)
	if err := json.NewDecoder(resp.Body).Decode(rdfDirList); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return rdfDirList, nil
}

// GetLocalOnlineRDFPorts gets all Online Ports for the given ONLINE RDF Director
func (c *Client) GetLocalOnlineRDFPorts(ctx context.Context, rdfDir string, localSymID string) (*types.RDFPortList, error) {
	defer c.TimeSpent("GetLocalOnlineRDFPorts", time.Now())
	if _, err := c.IsAllowedArray(localSymID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()

	URL := c.urlPrefix() + ReplicationX + SymmetrixX + localSymID + XRDFDIR + rdfDir + XRDFPORTONLINE

	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetLocalOnlineRDFPorts failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	rdfPortList := new(types.RDFPortList)
	if err := json.NewDecoder(resp.Body).Decode(rdfPortList); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return rdfPortList, nil
}

// GetRemoteRDFPortOnSAN gets Remote RDF Port on the SAN connected to this Local Array.
func (c *Client) GetRemoteRDFPortOnSAN(ctx context.Context, localSymID string, rdfDir string, rdfPort string) (*types.RemoteRDFPortDetails, error) {
	defer c.TimeSpent("GetRemoteRDFPortOnSAN", time.Now())
	if _, err := c.IsAllowedArray(localSymID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()

	URL := c.urlPrefix() + ReplicationX + SymmetrixX + localSymID + XRDFDIR + rdfDir + XRDFPORT + rdfPort + XREMOTEPORT

	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetRemoteRDFPortOnSAN failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	remoteRDFPortDetails := new(types.RemoteRDFPortDetails)
	if err := json.NewDecoder(resp.Body).Decode(remoteRDFPortDetails); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return remoteRDFPortDetails, nil
}

// GetLocalRDFPortDetails gets Local RDF Port Details which are Live on SAN.
func (c *Client) GetLocalRDFPortDetails(ctx context.Context, localSymID string, rdfDir string, rdfPort int) (*types.RDFPortDetails, error) {
	defer c.TimeSpent("GetLocalRDFPortDetails", time.Now())
	if _, err := c.IsAllowedArray(localSymID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()

	URL := c.urlPrefix() + ReplicationX + SymmetrixX + localSymID + XRDFDIR + rdfDir + XRDFPORT + strconv.Itoa(rdfPort)

	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetLocalRDFportDetails failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	LocalRDFPortDetails := new(types.RDFPortDetails)
	if err := json.NewDecoder(resp.Body).Decode(LocalRDFPortDetails); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return LocalRDFPortDetails, nil
}

// GetRDFGroupByID returns RDF group information given the RDF group number
func (c *Client) GetRDFGroupByID(ctx context.Context, symID, rdfGroupNo string) (*types.RDFGroup, error) {
	defer c.TimeSpent("GetRdfGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	URL := c.urlPrefix() + ReplicationX + SymmetrixX + symID + XRDFGroup + "/" + rdfGroupNo
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetRdfGroup failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	rdfGrpInfo := new(types.RDFGroup)
	if err := json.NewDecoder(resp.Body).Decode(rdfGrpInfo); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return rdfGrpInfo, nil
}

// GetRDFGroupList fetches all RDF group
func (c *Client) GetRDFGroupList(ctx context.Context, symID string, queryParams types.QueryParams) (*types.RDFGroupList, error) {
	defer c.TimeSpent("GetRdfGroupList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := c.urlPrefix() + ReplicationX + SymmetrixX + symID + XRDFGroup
	if queryParams != nil {
		URL += "?"
		for key, val := range queryParams {
			switch val := val.(type) {
			case int:
				URL += fmt.Sprintf("%s=%s", key, strconv.Itoa(val))
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
		log.Error("GetRdfGroupList failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	rdfGrpList := new(types.RDFGroupList)
	if err := json.NewDecoder(resp.Body).Decode(rdfGrpList); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return rdfGrpList, nil
}

// GetProtectedStorageGroup returns protected storage group given the storage group ID
func (c *Client) GetProtectedStorageGroup(ctx context.Context, symID, storageGroup string) (*types.RDFStorageGroup, error) {
	defer c.TimeSpent("GetProtectedStorageGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	URL := c.urlPrefix() + ReplicationX + SymmetrixX + symID + XStorageGroup + "/" + storageGroup
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetProtectedStorageGroup failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	rdfSgInfo := new(types.RDFStorageGroup)
	if err := json.NewDecoder(resp.Body).Decode(rdfSgInfo); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return rdfSgInfo, nil
}

// ExecuteCreateRDFGroup creates the RDF Group
func (c *Client) ExecuteCreateRDFGroup(ctx context.Context, symID string, CreateRDFPayload *types.RDFGroupCreate) error {
	defer c.TimeSpent("ExecuteCreateRDFGroup", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	URL := c.urlPrefix() + ReplicationX + SymmetrixX + symID + XRDFGroup
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Post(ctx, URL, c.getDefaultHeaders(), CreateRDFPayload, nil)
	if err != nil {
		log.Error("Error in ExecuteCreateRDFGroup: " + err.Error())
		return err
	}
	log.Debugf("sucessfully created RDF group")
	return nil
}

// ExecuteReplicationActionOnSG executes supported replication based actions on the protected SG
func (c *Client) ExecuteReplicationActionOnSG(ctx context.Context, symID, action, storageGroup, rdfGroup string, force, exemptConsistency, bias bool) error {
	defer c.TimeSpent("ExecuteReplicationActionOnSG", time.Now())

	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}

	modifyParam := &types.ModifySGRDFGroup{}

	switch action {
	case "Establish":
		actionParam := &types.Establish{
			Force:    force,
			SymForce: false,
			Star:     false,
			Hop2:     false,
		}
		if bias {
			actionParam.MetroBias = true
		}
		modifyParam = &types.ModifySGRDFGroup{
			Establish:       actionParam,
			Action:          action,
			ExecutionOption: types.ExecutionOptionSynchronous,
		}
	case "Suspend":
		actionParam := &types.Suspend{
			Force:      force,
			SymForce:   false,
			Star:       false,
			Hop2:       false,
			Immediate:  false,
			ConsExempt: exemptConsistency,
		}
		modifyParam = &types.ModifySGRDFGroup{
			Suspend:         actionParam,
			Action:          action,
			ExecutionOption: types.ExecutionOptionSynchronous,
		}
	case "Resume":
		actionParam := &types.Resume{
			Force:    force,
			SymForce: false,
			Star:     false,
			Hop2:     false,
			Remote:   false,
		}
		modifyParam = &types.ModifySGRDFGroup{
			Resume:          actionParam,
			Action:          action,
			ExecutionOption: types.ExecutionOptionSynchronous,
		}
	case "Failback":
		actionParam := &types.Failback{
			Force:    force,
			SymForce: false,
			Star:     false,
			Hop2:     false,
			Bypass:   false,
			Remote:   false,
		}
		modifyParam = &types.ModifySGRDFGroup{
			Failback:        actionParam,
			Action:          action,
			ExecutionOption: types.ExecutionOptionSynchronous,
		}
	case "Failover":
		actionParam := &types.Failover{
			Force:     force,
			SymForce:  false,
			Star:      false,
			Hop2:      false,
			Bypass:    false,
			Remote:    false,
			Immediate: false,
			Establish: false,
			Restore:   false,
		}
		modifyParam = &types.ModifySGRDFGroup{
			Failover:        actionParam,
			Action:          action,
			ExecutionOption: types.ExecutionOptionSynchronous,
		}
	case "Swap":
		actionParam := &types.Swap{
			Force:     force,
			SymForce:  false,
			Star:      false,
			Hop2:      false,
			Bypass:    false,
			HalfSwap:  false,
			RefreshR1: false,
			RefreshR2: false,
		}
		modifyParam = &types.ModifySGRDFGroup{
			Swap:            actionParam,
			Action:          action,
			ExecutionOption: types.ExecutionOptionSynchronous,
		}
	default:
		return fmt.Errorf("not a supported action on a protected storage group")
	}
	URL := c.urlPrefix() + ReplicationX + SymmetrixX + symID + XStorageGroup + "/" + storageGroup + XRDFGroup + "/" + rdfGroup
	fields := map[string]interface{}{
		http.MethodPut: URL,
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), modifyParam, nil)
	if err != nil {
		log.WithFields(fields).Error("Error in ExecuteReplicationActionOnSG: " + err.Error())
		return err
	}
	log.Info(fmt.Sprintf("Action (%s) on protected StorageGroup (%s) with RDF group (%s) is successful", action, storageGroup, rdfGroup))
	return nil
}

// GetCreateSGReplicaPayload returns a payload to create a storage group on remote array from local array and protect it with rdfgNo
func (c *Client) GetCreateSGReplicaPayload(remoteSymID string, rdfMode string, rdfgNo int, remoteSGName string, remoteServiceLevel string, establish, bias bool) *types.CreateSGSRDF {
	var payload *types.CreateSGSRDF
	switch rdfMode {
	case ASYNC:
		payload = &types.CreateSGSRDF{
			ReplicationMode:        "Asynchronous",
			RemoteSLO:              remoteServiceLevel,
			RemoteSymmID:           remoteSymID,
			RdfgNumber:             rdfgNo,
			RemoteStorageGroupName: remoteSGName,
			Establish:              establish,
			ExecutionOption:        types.ExecutionOptionSynchronous,
		}
	case SYNC:
		payload = &types.CreateSGSRDF{
			ReplicationMode:        "Synchronous",
			RemoteSLO:              remoteServiceLevel,
			RemoteSymmID:           remoteSymID,
			RdfgNumber:             rdfgNo,
			RemoteStorageGroupName: remoteSGName,
			Establish:              establish,
			ExecutionOption:        types.ExecutionOptionSynchronous,
		}
	case METRO:
		payload = &types.CreateSGSRDF{
			ReplicationMode:        "Active",
			RemoteSLO:              remoteServiceLevel,
			RemoteSymmID:           remoteSymID,
			RdfgNumber:             rdfgNo,
			RemoteStorageGroupName: remoteSGName,
			Establish:              establish,
			MetroBias:              bias,
			ExecutionOption:        types.ExecutionOptionSynchronous,
		}
	}
	return payload
}

// CreateSGReplica creates a storage group on remote array and protect them with given RDF Mode and a given source storage group
func (c *Client) CreateSGReplica(ctx context.Context, symID, remoteSymID, rdfMode, rdfGroupNo, sourceSG, remoteSGName, remoteServiceLevel string, bias bool) (*types.SGRDFInfo, error) {
	defer c.TimeSpent("CreateSGReplica", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	rdfgNo, _ := strconv.Atoi(rdfGroupNo)
	createSGReplicaPayload := c.GetCreateSGReplicaPayload(remoteSymID, rdfMode, rdfgNo, remoteSGName, remoteServiceLevel, true, bias)
	Debug = true
	ifDebugLogPayload(createSGReplicaPayload)
	URL := c.urlPrefix() + ReplicationX + SymmetrixX + symID + XStorageGroup + "/" + sourceSG + XRDFGroup

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodPost, URL, c.getDefaultHeaders(), createSGReplicaPayload)
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	rdfSG := &types.SGRDFInfo{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(rdfSG); err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully created SG replica for %s", sourceSG))
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return rdfSG, nil
}

// GetCreateRDFPairPayload returns payload for adding a replication pair based on replication mode
func (c *Client) GetCreateRDFPairPayload(devList types.LocalDeviceListCriteria, rdfMode, rdfType string, establish, exemptConsistency bool) *types.CreateRDFPair {
	var payload *types.CreateRDFPair
	switch rdfMode {
	case ASYNC:
		payload = &types.CreateRDFPair{
			RdfMode:                 "Asynchronous",
			RdfType:                 rdfType,
			Establish:               establish,
			Exempt:                  exemptConsistency,
			LocalDeviceListCriteria: &devList,
			ExecutionOption:         types.ExecutionOptionSynchronous,
		}
	case SYNC:
		payload = &types.CreateRDFPair{
			RdfMode:                 "Synchronous",
			RdfType:                 rdfType,
			Establish:               establish,
			Exempt:                  exemptConsistency,
			LocalDeviceListCriteria: &devList,
			ExecutionOption:         types.ExecutionOptionSynchronous,
		}
	case METRO:
		payload = &types.CreateRDFPair{
			RdfMode:                 "Active",
			RdfType:                 "RDF1",
			Bias:                    true,
			Establish:               establish,
			Exempt:                  exemptConsistency,
			LocalDeviceListCriteria: &devList,
			ExecutionOption:         types.ExecutionOptionSynchronous,
		}
	}
	return payload
}

// CreateRDFPair creates an RDF device pair in the given RDF group
func (c *Client) CreateRDFPair(ctx context.Context, symID, rdfGroupNo, deviceID, rdfMode, rdfType string, establish, exemptConsistency bool) (*types.RDFDevicePairList, error) {
	defer c.TimeSpent("CreateRDFPair", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	var deviceList []string
	deviceList = append(deviceList, deviceID)
	devList := types.LocalDeviceListCriteria{
		LocalDeviceList: deviceList,
	}
	createPairPayload := c.GetCreateRDFPairPayload(devList, rdfMode, rdfType, establish, exemptConsistency)
	Debug = true
	ifDebugLogPayload(createPairPayload)
	URL := c.urlPrefix() + ReplicationX + SymmetrixX + symID + XRDFGroup + "/" + rdfGroupNo + XVolume + "/" + deviceID

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodPost, URL, c.getDefaultHeaders(), createPairPayload)
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	rdfPairList := &types.RDFDevicePairList{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(rdfPairList); err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("Successfully created volume replica for %s", deviceID))
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return rdfPairList, nil
}

// GetRDFDevicePairInfo returns RDF volume information
func (c *Client) GetRDFDevicePairInfo(ctx context.Context, symID, rdfGroup, volumeID string) (*types.RDFDevicePair, error) {
	defer c.TimeSpent("GetRDFDevicePairInfo", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	URL := c.urlPrefix() + ReplicationX + SymmetrixX + symID + XRDFGroup + "/" + rdfGroup + XVolume + "/" + volumeID
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetRDFDevicePairInfo failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	rdfDevPairInfo := new(types.RDFDevicePair)
	if err := json.NewDecoder(resp.Body).Decode(rdfDevPairInfo); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return rdfDevPairInfo, nil
}

// GetStorageGroupRDFInfo returns the of RDF info of protected storage group
func (c *Client) GetStorageGroupRDFInfo(ctx context.Context, symID, sgName, rdfGroupNo string) (*types.StorageGroupRDFG, error) {
	defer c.TimeSpent("GetStorageGroupRDFInfo", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	URL := c.urlPrefix() + ReplicationX + SymmetrixX + symID + XStorageGroup + "/" + sgName + XRDFGroup + "/" + rdfGroupNo
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetStorageGroupRDFInfo failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	sgRdfInfo := new(types.StorageGroupRDFG)
	if err := json.NewDecoder(resp.Body).Decode(sgRdfInfo); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return sgRdfInfo, nil
}
