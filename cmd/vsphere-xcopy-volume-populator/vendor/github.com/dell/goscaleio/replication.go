/*
 * Copyright © 2020-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package goscaleio

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	types "github.com/dell/goscaleio/types/v1"
)

// List of consistency group states.
const (
	Inconsistent        string = "Inconsistent"
	Consistent          string = "Consistent"
	ConsistentPending   string = "ConsistentPending"
	Invalid             string = "Invalid"
	PartiallyConsistent string = "PartiallyConsistent"
)

// PeerMDM encpsulates a PeerMDM type and a client.
type PeerMDM struct {
	PeerMDM *types.PeerMDM
	client  *Client
}

// NewPeerMDM creates a PeerMDM from a types.PeerMDM and a client.
func NewPeerMDM(client *Client, peerMDM *types.PeerMDM) *PeerMDM {
	newPeerMDM := &PeerMDM{
		client:  client,
		PeerMDM: peerMDM,
	}
	return newPeerMDM
}

// GetPeerMDMs returns a list of peer MDMs know to the System
func (c *Client) GetPeerMDMs() ([]*types.PeerMDM, error) {
	defer TimeSpent("GetPeerMDMs", time.Now())

	path := "/api/types/PeerMdm/instances"
	var peerMdms []*types.PeerMDM

	err := c.getJSONWithRetry(http.MethodGet, path, nil, &peerMdms)
	return peerMdms, err
}

// GetPeerMDM returns a specific peer MDM
func (c *Client) GetPeerMDM(id string) (*types.PeerMDM, error) {
	defer TimeSpent("GetPeerMDM", time.Now())

	path := "/api/instances/PeerMdm::" + id
	var peerMdm *types.PeerMDM

	err := c.getJSONWithRetry(http.MethodGet, path, nil, &peerMdm)
	return peerMdm, err
}

// ModifyPeerMdmIP updates a Peer MDM Ips
func (c *Client) ModifyPeerMdmIP(id string, ips []string) error {
	defer TimeSpent("ModifyPeerMdmIP", time.Now())
	// Format into the strucutre that the API expects
	var ipMap []map[string]interface{}
	for _, ip := range ips {
		ipMap = append(ipMap, map[string]interface{}{"hostName": ip})
	}
	param := types.ModifyPeerMdmIPParam{
		NewPeerMDMIps: ipMap,
	}
	path := "/api/instances/PeerMdm::" + id + "/action/modifyPeerMdmIp"

	if err := c.getJSONWithRetry(http.MethodPost, path, param, nil); err != nil {
		fmt.Printf("c.getJSONWithRetry(http.MethodPost, path, param, nil) returned %s", err)
		return err
	}

	return nil
}

// ModifyPeerMdmName updates a Peer MDM Name
func (c *Client) ModifyPeerMdmName(id string, name *types.ModifyPeerMDMNameParam) error {
	defer TimeSpent("ModifyPeerMdmName", time.Now())

	path := "/api/instances/PeerMdm::" + id + "/action/modifyPeerMdmName"

	if err := c.getJSONWithRetry(http.MethodPost, path, name, nil); err != nil {
		fmt.Printf("c.getJSONWithRetry(http.MethodPost, path, name, nil) returned %s", err)
		return err
	}

	return nil
}

// ModifyPeerMdmPort updates a Peer MDM Port
func (c *Client) ModifyPeerMdmPort(id string, port *types.ModifyPeerMDMPortParam) error {
	defer TimeSpent("ModifyPeerMdmPort", time.Now())

	path := "/api/instances/PeerMdm::" + id + "/action/modifyPeerMdmPort"

	if err := c.getJSONWithRetry(http.MethodPost, path, port, nil); err != nil {
		fmt.Printf("c.getJSONWithRetry(http.MethodPost, path, port, nil) returned %s", err)
		return err
	}

	return nil
}

// ModifyPeerMdmPerformanceParameters updates a Peer MDM Performance Parameters
func (c *Client) ModifyPeerMdmPerformanceParameters(id string, param *types.ModifyPeerMdmPerformanceParametersParam) error {
	defer TimeSpent("ModifyPeerMdmPerformanceParameters", time.Now())

	path := "/api/instances/PeerMdm::" + id + "/action/setPeerMdmPerformanceParameters"

	if err := c.getJSONWithRetry(http.MethodPost, path, param, nil); err != nil {
		fmt.Printf("c.getJSONWithRetry(http.MethodPost, path, param, nil) returned %s", err)
		return err
	}

	return nil
}

// AddPeerMdm Adds a Peer MDM
func (c *Client) AddPeerMdm(param *types.AddPeerMdm) (*types.PeerMDM, error) {
	defer TimeSpent("AddPeerMdm", time.Now())
	if param.PeerSystemID == "" || len(param.PeerSystemIps) == 0 {
		return nil, errors.New("PeerSystemID and PeerSystemIps are required")
	}
	path := "/api/types/PeerMdm/instances"
	peerMdm := &types.PeerMDM{}
	var ipMap []map[string]interface{}
	for _, ip := range param.PeerSystemIps {
		ipMap = append(ipMap, map[string]interface{}{"hostName": ip})
	}
	paramCreate := types.AddPeerMdmParam{
		PeerSystemID:  param.PeerSystemID,
		PeerSystemIps: ipMap,
		Port:          param.Port,
		Name:          param.Name,
	}

	if err := c.getJSONWithRetry(http.MethodPost, path, paramCreate, peerMdm); err != nil {
		fmt.Printf("c.getJSONWithRetry(http.MethodPost, path, paramCreate, peerMdm) returned %s", err)
		return nil, err
	}

	return peerMdm, nil
}

// RemovePeerMdm removes a Peer MDM
func (c *Client) RemovePeerMdm(id string) error {
	defer TimeSpent("RemovePeerMdm", time.Now())

	path := "/api/instances/PeerMdm::" + id + "/action/removePeerMdm"
	params := types.EmptyPayload{}
	if err := c.getJSONWithRetry(http.MethodPost, path, params, nil); err != nil {
		fmt.Printf("c.getJSONWithRetry(http.MethodPost, path, params, nil) returned %s", err)
		return err
	}

	return nil
}

// ReplicationConsistencyGroup encpsulates a types.ReplicationConsistencyGroup and a client.
type ReplicationConsistencyGroup struct {
	ReplicationConsistencyGroup *types.ReplicationConsistencyGroup
	client                      *Client
}

// NewReplicationConsistencyGroup creates a new ReplicationConsistencyGroup.
func NewReplicationConsistencyGroup(client *Client) *ReplicationConsistencyGroup {
	rcg := &ReplicationConsistencyGroup{
		client:                      client,
		ReplicationConsistencyGroup: &types.ReplicationConsistencyGroup{},
	}
	return rcg
}

// ReplicationPair encpsulates a types.ReplicationPair and a client.
type ReplicationPair struct {
	ReplicaitonPair *types.ReplicationPair
	client          *Client
}

// NewReplicationPair creates a new ReplicationConsistencyGroup.
func NewReplicationPair(client *Client) *ReplicationPair {
	rcg := &ReplicationPair{
		client:          client,
		ReplicaitonPair: &types.ReplicationPair{},
	}
	return rcg
}

// GetReplicationConsistencyGroups returns a list of the ReplicationConsistencyGroups
func (c *Client) GetReplicationConsistencyGroups() ([]*types.ReplicationConsistencyGroup, error) {
	defer TimeSpent("GetReplicationConsistencyGroups", time.Now())

	uri := "/api/types/ReplicationConsistencyGroup/instances"
	var rcgs []*types.ReplicationConsistencyGroup

	err := c.getJSONWithRetry(http.MethodGet, uri, nil, &rcgs)
	return rcgs, err
}

// GetReplicationConsistencyGroupByID returns a specified ReplicationConsistencyGroup
func (c *Client) GetReplicationConsistencyGroupByID(groupID string) (*types.ReplicationConsistencyGroup, error) {
	defer TimeSpent("GetReplicationConsistencyGroupById", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + groupID
	var group *types.ReplicationConsistencyGroup

	err := c.getJSONWithRetry(http.MethodGet, uri, nil, &group)
	return group, err
}

// CreateReplicationConsistencyGroup creates a ReplicationConsistencyGroup on the array
func (c *Client) CreateReplicationConsistencyGroup(rcg *types.ReplicationConsistencyGroupCreatePayload) (*types.ReplicationConsistencyGroupResp, error) {
	defer TimeSpent("CreateReplicationConsistencyGroup", time.Now())

	if rcg.RpoInSeconds == "" || rcg.ProtectionDomainID == "" || rcg.RemoteProtectionDomainID == "" {
		return nil, errors.New("RpoInSeconds, ProtectionDomainId, and RemoteProtectionDomainId are required")
	}

	if rcg.DestinationSystemID == "" && rcg.PeerMdmID == "" {
		return nil, errors.New("either DestinationSystemId or PeerMdmId are required")
	}

	path := "/api/types/ReplicationConsistencyGroup/instances"
	rcgResp := &types.ReplicationConsistencyGroupResp{}

	err := c.getJSONWithRetry(http.MethodPost, path, rcg, rcgResp)
	if err != nil {
		fmt.Printf("c.getJSONWithRetry(http.MethodPost, path, rcg, rcgResp) returned %s", err)
		return nil, err
	}
	return rcgResp, nil
}

// RemoveReplicationConsistencyGroup removes a replication consistency group
// At this point I don't know when forceIgnoreConsistency might be required.
func (rcg *ReplicationConsistencyGroup) RemoveReplicationConsistencyGroup(forceIgnoreConsistency bool) error {
	defer TimeSpent("RemoveReplicationConsistencyGroup", time.Now())

	link, err := GetLink(rcg.ReplicationConsistencyGroup.Links, "self")
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%v/action/removeReplicationConsistencyGroup", link.HREF)

	removeRCGParam := &types.RemoveReplicationConsistencyGroupParam{}
	if forceIgnoreConsistency {
		removeRCGParam.ForceIgnoreConsistency = "True"
	}

	err = rcg.client.getJSONWithRetry(http.MethodPost, path, removeRCGParam, nil)
	return err
}

// FreezeReplicationConsistencyGroup sets the ReplicationConsistencyGroup into a freeze state
func (rcg *ReplicationConsistencyGroup) FreezeReplicationConsistencyGroup(id string) error {
	defer TimeSpent("FreezeReplicationConsistencyGroup", time.Now())

	params := types.EmptyPayload{}
	path := "/api/instances/ReplicationConsistencyGroup::" + id + "/action/freezeApplyReplicationConsistencyGroup"

	err := rcg.client.getJSONWithRetry(http.MethodPost, path, params, nil)
	return err
}

// UnfreezeReplicationConsistencyGroup sets the ReplicationConsistencyGroup into a Unfreeze state
func (rcg *ReplicationConsistencyGroup) UnfreezeReplicationConsistencyGroup() error {
	defer TimeSpent("UnfreezeReplicationConsistencyGroup", time.Now())

	params := types.EmptyPayload{}
	path := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/unfreezeApplyReplicationConsistencyGroup"

	err := rcg.client.getJSONWithRetry(http.MethodPost, path, params, nil)
	return err
}

// CreateReplicationPair creates a ReplicationPair on the desired ReplicaitonConsistencyGroup
func (c *Client) CreateReplicationPair(rp *types.QueryReplicationPair) (*types.ReplicationPair, error) {
	defer TimeSpent("CreateReplicationPair", time.Now())

	if rp.CopyType == "" || rp.SourceVolumeID == "" || rp.DestinationVolumeID == "" || rp.ReplicationConsistencyGroupID == "" {
		return nil, errors.New("CopyType, SourceVolumeID, DestinationVolumeID, and ReplicationConsistencyGroupID are required")
	}

	path := "/api/types/ReplicationPair/instances"
	rpResp := &types.ReplicationPair{}

	if err := c.getJSONWithRetry(http.MethodPost, path, rp, rpResp); err != nil {
		fmt.Printf("c.getJSONWithRetry(http.MethodPost, path, rp, rpResp) returned %s", err)
		return nil, err
	}

	return rpResp, nil
}

// RemoveReplicationPair removes the desired replication pair.
func (rp *ReplicationPair) RemoveReplicationPair(force bool) (*types.ReplicationPair, error) {
	defer TimeSpent("RemoveReplicationPair", time.Now())

	uri := "/api/instances/ReplicationPair::" + rp.ReplicaitonPair.ID + "/action/removeReplicationPair"
	resp := &types.ReplicationPair{}
	param := &types.RemoveReplicationPair{
		Force: "false",
	}
	if force {
		param.Force = "true"
	}

	if err := rp.client.getJSONWithRetry(http.MethodPost, uri, param, resp); err != nil {
		fmt.Printf("c.getJSONWithRetry(http.MethodPost, path, rp, pair) returned %s", err)
		return nil, err
	}

	return resp, nil
}

// GetReplicationPairStatistics returns the statistics of the desired ReplicaitonPair.
func (rp *ReplicationPair) GetReplicationPairStatistics() (*types.QueryReplicationPairStatistics, error) {
	defer TimeSpent("GetReplicationPairStatistics", time.Now())

	path := "/api/instances/ReplicationPair::" + rp.ReplicaitonPair.ID + "/relationships/Statistics"
	rpResp := &types.QueryReplicationPairStatistics{}

	err := rp.client.getJSONWithRetry(http.MethodGet, path, nil, &rpResp)
	return rpResp, err
}

// GetAllReplicationPairs returns a list all replication pairs on the system.
func (c *Client) GetAllReplicationPairs() ([]*types.ReplicationPair, error) {
	defer TimeSpent("GetReplicationPairs", time.Now())

	path := "/api/types/ReplicationPair/instances"

	var pairs []*types.ReplicationPair
	err := c.getJSONWithRetry(http.MethodGet, path, nil, &pairs)
	return pairs, err
}

// GetReplicationPair returns a specific replication pair on the system.
func (c *Client) GetReplicationPair(id string) (*types.ReplicationPair, error) {
	defer TimeSpent("GetReplicationPair", time.Now())

	path := "/api/instances/ReplicationPair::" + id

	var pair *types.ReplicationPair
	err := c.getJSONWithRetry(http.MethodGet, path, nil, &pair)
	return pair, err
}

// PausePairInitialCopy pauses the initial copy of the replication pair.
func (c *Client) PausePairInitialCopy(id string) (*types.ReplicationPair, error) {
	defer TimeSpent("PausePairInitialCopy", time.Now())

	path := "/api/instances/ReplicationPair::" + id + "/action/pausePairInitialCopy"

	var pair *types.ReplicationPair
	err := c.getJSONWithRetry(http.MethodPost, path, types.EmptyPayload{}, &pair)
	return pair, err
}

// ResumePairInitialCopy resumes the initial copy of the replication pair.
func (c *Client) ResumePairInitialCopy(id string) (*types.ReplicationPair, error) {
	defer TimeSpent("ResumePairInitialCopy", time.Now())

	path := "/api/instances/ReplicationPair::" + id + "/action/resumePairInitialCopy"

	var pair *types.ReplicationPair
	err := c.getJSONWithRetry(http.MethodPost, path, types.EmptyPayload{}, &pair)
	return pair, err
}

// GetReplicationPairs returns a list of replication pairs associated to the rcg.
func (rcg *ReplicationConsistencyGroup) GetReplicationPairs() ([]*types.ReplicationPair, error) {
	defer TimeSpent("GetReplicationPairs", time.Now())

	path := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/relationships/ReplicationPair"

	var pairs []*types.ReplicationPair
	err := rcg.client.getJSONWithRetry(http.MethodGet, path, nil, &pairs)
	return pairs, err
}

// CreateReplicationConsistencyGroupSnapshot creates a snapshot of the ReplicationConsistencyGroup on the target array.
func (rcg *ReplicationConsistencyGroup) CreateReplicationConsistencyGroupSnapshot() (*types.CreateReplicationConsistencyGroupSnapshotResp, error) {
	defer TimeSpent("GetReplicationPairs", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/createReplicationConsistencyGroupSnapshots"

	resp := &types.CreateReplicationConsistencyGroupSnapshotResp{}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, types.EmptyPayload{}, resp)
	return resp, err
}

// ExecuteFailoverOnReplicationGroup sets the ReplicationconsistencyGroup into a failover state.
func (rcg *ReplicationConsistencyGroup) ExecuteFailoverOnReplicationGroup() error {
	defer TimeSpent("ExecuteFailoverOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/failoverReplicationConsistencyGroup"
	param := types.EmptyPayload{}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// ExecuteSwitchoverOnReplicationGroup sets the ReplicationconsistencyGroup into a switchover state.
func (rcg *ReplicationConsistencyGroup) ExecuteSwitchoverOnReplicationGroup(_ bool) error {
	defer TimeSpent("ExecuteSwitchoverOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/switchoverReplicationConsistencyGroup"
	// API is incorrect. No params needed.
	param := types.EmptyPayload{}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// ExecuteRestoreOnReplicationGroup restores the ReplicationConsistencyGroup from a failover/switchover state.
func (rcg *ReplicationConsistencyGroup) ExecuteRestoreOnReplicationGroup() error {
	defer TimeSpent("ExecuteRestoreOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/restoreReplicationConsistencyGroup"
	param := types.EmptyPayload{}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// ExecuteReverseOnReplicationGroup reverses the direction of replication from a failover/switchover state.
func (rcg *ReplicationConsistencyGroup) ExecuteReverseOnReplicationGroup() error {
	defer TimeSpent("ExecuteReverseOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/reverseReplicationConsistencyGroup"
	param := types.EmptyPayload{}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// ExecutePauseOnReplicationGroup pauses the replication of the ConsistencyGroup.
func (rcg *ReplicationConsistencyGroup) ExecutePauseOnReplicationGroup() error {
	defer TimeSpent("ExecutePauseOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/pauseReplicationConsistencyGroup"
	param := types.PauseReplicationConsistencyGroup{
		PauseMode: string(types.StopDataTransfer),
	}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// ExecuteResumeOnReplicationGroup resumes the ConsistencyGroup when it is in a Paused state.
func (rcg *ReplicationConsistencyGroup) ExecuteResumeOnReplicationGroup() error {
	defer TimeSpent("ExecuteResumeOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/resumeReplicationConsistencyGroup"
	param := types.EmptyPayload{}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// ExecuteSyncOnReplicationGroup forces a synce on the ConsistencyGroup.
func (rcg *ReplicationConsistencyGroup) ExecuteSyncOnReplicationGroup() (*types.SynchronizationResponse, error) {
	defer TimeSpent("ExecuteSyncOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/syncNowReplicationConsistencyGroup"
	param := types.EmptyPayload{}
	resp := &types.SynchronizationResponse{}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, resp)
	return resp, err
}

// SetRPOOnReplicationGroup on the ConsistencyGroup.
func (rcg *ReplicationConsistencyGroup) SetRPOOnReplicationGroup(param types.SetRPOReplicationConsistencyGroup) error {
	defer TimeSpent("SetRPOOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/ModifyReplicationConsistencyGroupRpo"

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// SetTargetVolumeAccessModeOnReplicationGroup on the ConsistencyGroup.
func (rcg *ReplicationConsistencyGroup) SetTargetVolumeAccessModeOnReplicationGroup(param types.SetTargetVolumeAccessModeOnReplicationGroup) error {
	defer TimeSpent("SetTargetVolumeAccessModeOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/modifyReplicationConsistencyGroupTargetVolumeAccessMode"

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// SetNewNameOnReplicationGroup on the ConsistencyGroup.
func (rcg *ReplicationConsistencyGroup) SetNewNameOnReplicationGroup(param types.SetNewNameOnReplicationGroup) error {
	defer TimeSpent("SetNewNameOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/renameReplicationConsistencyGroup"

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// ExecuteConsistentOnReplicationGroup on the ConsistencyGroup.
func (rcg *ReplicationConsistencyGroup) ExecuteConsistentOnReplicationGroup() error {
	defer TimeSpent("ExecuteConsistentOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/setReplicationConsistencyGroupConsistent"
	param := types.EmptyPayload{}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// ExecuteInconsistentOnReplicationGroup on the ConsistencyGroup.
func (rcg *ReplicationConsistencyGroup) ExecuteInconsistentOnReplicationGroup() error {
	defer TimeSpent("ExecuteInconsistentOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/setReplicationConsistencyGroupInconsistent"
	param := types.EmptyPayload{}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// ExecuteActivateOnReplicationGroup on the ConsistencyGroup.
func (rcg *ReplicationConsistencyGroup) ExecuteActivateOnReplicationGroup() error {
	defer TimeSpent("ExecuteActivateOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/activateReplicationConsistencyGroup"
	param := types.EmptyPayload{}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// ExecuteTerminateOnReplicationGroup on the ConsistencyGroup.
func (rcg *ReplicationConsistencyGroup) ExecuteTerminateOnReplicationGroup() error {
	defer TimeSpent("ExecuteTerminateOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/terminateReplicationConsistencyGroup"
	param := types.EmptyPayload{}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}

// GetSyncStateOnReplicationGroup returns the sync status of the ReplicaitonConsistencyGroup.
func (rcg *ReplicationConsistencyGroup) GetSyncStateOnReplicationGroup(syncKey string) error {
	defer TimeSpent("ExecuteSyncOnReplicationGroup", time.Now())

	uri := "/api/instances/ReplicationConsistencyGroup::" + rcg.ReplicationConsistencyGroup.ID + "/action/querySyncNowReplicationConsistencyGroup"
	param := types.QuerySyncNowRequest{
		SyncNowKey: syncKey,
	}

	err := rcg.client.getJSONWithRetry(http.MethodPost, uri, param, nil)
	return err
}
