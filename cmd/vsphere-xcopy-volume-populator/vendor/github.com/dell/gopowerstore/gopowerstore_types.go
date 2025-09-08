/*
 *
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

package gopowerstore

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/dell/gopowerstore/api"
)

// RequestConfig represents options for request
type RequestConfig api.RequestConfig

// naslimitRegex is used to check if the error message contains a limit of file systems for the NAS server
var naslimitRegex = regexp.MustCompile(`limit of \d+ file systems for the NAS server`)

// RenderRequestConfig returns internal struct with request config
func (rc RequestConfig) RenderRequestConfig() api.RequestConfig {
	return api.RequestConfig(rc)
}

// CreateResponse create response
type CreateResponse struct {
	// Unique identifier of the new instance created.
	ID string `json:"id,omitempty"`
}

// EmptyResponse is response without content
type EmptyResponse string

// APIError represents API error
type APIError struct {
	*api.ErrorMsg
}

// NewAPIError returns pointer to new APIError
func NewAPIError() *APIError {
	return &APIError{&api.ErrorMsg{}}
}

// WrapErr converts internal error type to public
func WrapErr(err error) error {
	errorMsg, ok := err.(*api.ErrorMsg)
	if ok {
		err = APIError{errorMsg}
	}
	return err
}

// NotFound returns true if API error indicate that volume is not exists
func (err *APIError) NotFound() bool {
	return err.StatusCode == http.StatusNotFound
}

// VolumeNameIsAlreadyUse returns true if API error indicate that volume name is already in use
func (err *APIError) VolumeNameIsAlreadyUse() bool {
	return err.StatusCode == http.StatusUnprocessableEntity || err.StatusCode == http.StatusInternalServerError
}

// SnapshotNameIsAlreadyUse returns true if API error indicate that snapshot name is already in use
func (err *APIError) SnapshotNameIsAlreadyUse() bool {
	return err.StatusCode == http.StatusBadRequest
}

// FSNameIsAlreadyUse returns true if API error indicate that fs name is already in use
func (err *APIError) FSNameIsAlreadyUse() bool {
	return err.StatusCode == http.StatusUnprocessableEntity
}

// HostIsNotAttachedToVolume returns true if API error indicate that host is not attached to volume
func (err *APIError) HostIsNotAttachedToVolume() bool {
	return err.StatusCode == http.StatusBadRequest
}

// VolumeIsNotAttachedToHost returns true if API error indicate that volume is not attached to host
func (err *APIError) VolumeIsNotAttachedToHost() bool {
	return err.StatusCode == http.StatusBadRequest
}

// HostIsNotExist returns true if API error indicate that host is not exists
func (err *APIError) HostIsNotExist() bool {
	return err.StatusCode == http.StatusNotFound || err.StatusCode == http.StatusBadRequest
}

// BadRange returns true if API error indicate that request was submitted with invalid range
func (err *APIError) BadRange() bool {
	return err.StatusCode == http.StatusRequestedRangeNotSatisfiable
}

// VolumeAttachedToHost returns true if API error indicate that operation can't be complete because
// volume is attached to host
func (err *APIError) VolumeAttachedToHost() bool {
	return err.StatusCode == http.StatusUnprocessableEntity
}

// VolumeDetachedFromHost returns true if API error indicate that volume is detached from host
func (err *APIError) VolumeDetachedFromHost() bool {
	return err.StatusCode == http.StatusUnprocessableEntity
}

// HostAlreadyRemovedFromNFSExport returns true if API error indicate that operation can't be complete because
// host ip already removed from nfs export access
func (err *APIError) HostAlreadyRemovedFromNFSExport() bool {
	return err.StatusCode == http.StatusBadRequest
}

// HostAlreadyPresentInNFSExport returns true if API error indicate that operation can't be complete because
// host ip already present in nfs export access
func (err *APIError) HostAlreadyPresentInNFSExport() bool {
	return err.StatusCode == http.StatusBadRequest
}

// UnableToFailoverFromDestination returns true if API error indicate that operation can't be complete because
// it is impossible to failover from Destination
func (err *APIError) UnableToFailoverFromDestination() bool {
	return err.StatusCode == http.StatusBadRequest
}

// ReplicationSessionAlreadyCreated returns true if API error indicate that replication session has already been created
func (err *APIError) ReplicationSessionAlreadyCreated() bool {
	return err.StatusCode == http.StatusBadRequest
}

// VolumeAlreadyRemovedFromVolumeGroup returns true if API error indicate that volume is not part of the volume group
func (err *APIError) VolumeAlreadyRemovedFromVolumeGroup() bool {
	return err.StatusCode == http.StatusUnprocessableEntity && strings.Contains(err.Message, "not part")
}

// FSCreationLimitReached returns true if API error indicate that file system creation limit has been reached
func (err *APIError) FSCreationLimitReached() bool {
	return err.StatusCode == http.StatusUnprocessableEntity && naslimitRegex.MatchString(err.Message)
}

// NewNotFoundError returns new VolumeIsNotExistError
func NewNotFoundError() APIError {
	return notFoundError()
}

// NewHostIsNotExistError returns new HostIsNotExistError
func NewHostIsNotExistError() APIError {
	return notFoundError()
}

// NewHostIsNotAttachedToVolume returns new HostIsNotAttachedToVolume error
func NewHostIsNotAttachedToVolume() APIError {
	apiError := APIError{&api.ErrorMsg{}}
	apiError.StatusCode = http.StatusBadRequest
	return apiError
}

// NewVolumeAttachedToHostError returns new VolumeAttachedToHost error
func NewVolumeAttachedToHostError() APIError {
	apiError := APIError{&api.ErrorMsg{}}
	apiError.StatusCode = http.StatusUnprocessableEntity
	return apiError
}

func notFoundError() APIError {
	apiError := APIError{&api.ErrorMsg{}}
	apiError.StatusCode = http.StatusNotFound
	return apiError
}

func replicationRuleNotExists() APIError {
	apiError := APIError{&api.ErrorMsg{}}
	apiError.StatusCode = http.StatusNotFound
	return apiError
}

func snapshotRuleNotExists() APIError {
	apiError := APIError{&api.ErrorMsg{}}
	apiError.StatusCode = http.StatusNotFound
	return apiError
}

func protectionPolicyNotExists() APIError {
	apiError := APIError{&api.ErrorMsg{}}
	apiError.StatusCode = http.StatusNotFound
	return apiError
}

func replicationGroupNotExists() APIError {
	apiError := APIError{&api.ErrorMsg{}}
	apiError.StatusCode = http.StatusNotFound
	return apiError
}
