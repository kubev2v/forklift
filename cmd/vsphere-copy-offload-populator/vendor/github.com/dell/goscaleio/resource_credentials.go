/*
 * Copyright © 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"fmt"
	"net/http"
	"time"

	types "github.com/dell/goscaleio/types/v1"
)

// GetResourceCredentials returns all the resource credentials
func (s *System) GetResourceCredentials() (*types.ResourceCredentials, error) {
	defer TimeSpent("GetResourceCredentials", time.Now())

	path := fmt.Sprintf(
		"/api/v1/Credential")

	var credentialsResult types.ResourceCredentials
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &credentialsResult)
	if err != nil {
		return nil, err
	}

	return &credentialsResult, nil
}

// GetResourceCredential returns a specific credential using resource credential ID
func (s *System) GetResourceCredential(id string) (*types.ResourceCredential, error) {
	defer TimeSpent("GetResourceCredential", time.Now())

	path := fmt.Sprintf(
		"/api/v1/Credential/%v", id)

	var credentialResult types.ResourceCredential
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

func validateNodeCred(body types.ServerCredential) (*types.ServerCredential, error) {
	// If the SNMPv2CommunityString and Snmpv3SecurityName are not empty, Default to SNMPv2Protocol
	if body.SNMPv2CommunityString == "" && body.SNMPv3SecurityName == "" {
		body.SNMPv2CommunityString = "public"
		body.SNMPv2Protocol = "SSH"
	} else if body.SNMPv2CommunityString != "" {
		body.SNMPv2Protocol = "SSH"
	} else {
		switch body.SNMPv3SecurityLevel {
		case "1":
		// No validations needed
		case "2":
			// MD5 Auth Password needes to be set
			if body.SNMPv3MD5AuthenticationPassword == "" {
				return nil, fmt.Errorf("If SNMPv3Security level is 2 the MD5AuthentiacionPassword must be set")
			}
		case "3":
			// MD5 Auth Password and DESPrivatePassword needes to be set
			if body.SNMPv3MD5AuthenticationPassword == "" || body.SNMPv3DesPrivatePassword == "" {
				return nil, fmt.Errorf("If SNMPv3Security level is 3 the MD5AuthentiacionPassword and DESPrivatePassword must be set")
			}
		default:
			return nil, fmt.Errorf("invalid SNMPv3SecurityLevel: %v, should be 1,2 or 3", body.SNMPv3SecurityLevel)
		}
	}

	if (body.SSHPrivateKey != "" && body.KeyPairName == "") || (body.SSHPrivateKey == "" && body.KeyPairName != "") {
		return nil, fmt.Errorf("If using an SSHPrivateKey then both KeyPairName and SSHPrivateKey must be set")
	}

	return &body, nil
}

// CreateNodeResourceCredential creates a new Resource Credential
func (s *System) CreateNodeResourceCredential(body types.ServerCredential) (*types.ResourceCredential, error) {
	valBody, errVal := validateNodeCred(body)
	if errVal != nil {
		return nil, errVal
	}
	fullBody := types.NodeCredentialWrapper{
		ServerCredential: *valBody,
	}
	defer TimeSpent("CreateNodeResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential")

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPost, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// ModifyNodeResourceCredential creates a new Resource Credential
func (s *System) ModifyNodeResourceCredential(body types.ServerCredential, id string) (*types.ResourceCredential, error) {
	valBody, errVal := validateNodeCred(body)
	if errVal != nil {
		return nil, errVal
	}

	fullBody := types.NodeCredentialWrapper{
		ServerCredential: *valBody,
	}
	defer TimeSpent("ModifyNodeResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential/%v", id)

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPut, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// CreateSwitchResourceCredential creates a new Resource Credential
func (s *System) CreateSwitchResourceCredential(body types.IomCredential) (*types.ResourceCredential, error) {
	// Validations
	if (body.SSHPrivateKey != "" && body.KeyPairName == "") || (body.SSHPrivateKey == "" && body.KeyPairName != "") {
		return nil, fmt.Errorf("If using an SSHPrivateKey then both KeyPairName and SSHPrivateKey must be set")
	}
	// Set to default SNMPv2CommunityString if empty, set the SNMPv2Protocol to "SSH"
	if body.SNMPv2CommunityString == "" {
		body.SNMPv2CommunityString = "public"
	}
	body.SNMPv2Protocol = "SSH"

	fullBody := types.SwitchCredentialWrapper{
		IomCredential: body,
	}
	defer TimeSpent("CreateSwitchResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential")

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPost, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// ModifySwitchResourceCredential creates a new Resource Credential
func (s *System) ModifySwitchResourceCredential(body types.IomCredential, id string) (*types.ResourceCredential, error) {
	// Validations
	if (body.SSHPrivateKey != "" && body.KeyPairName == "") || (body.SSHPrivateKey == "" && body.KeyPairName != "") {
		return nil, fmt.Errorf("If using an SSHPrivateKey then both KeyPairName and SSHPrivateKey must be set")
	}
	// Set to default SNMPv2CommunityString if empty, set the SNMPv2Protocol to "SSH"
	if body.SNMPv2CommunityString == "" {
		body.SNMPv2CommunityString = "public"
	}
	body.SNMPv2Protocol = "SSH"

	fullBody := types.SwitchCredentialWrapper{
		IomCredential: body,
	}
	defer TimeSpent("ModifySwitchResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential/%v", id)

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPut, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// CreateVCenterResourceCredential creates a new Resource Credential
func (s *System) CreateVCenterResourceCredential(body types.VCenterCredential) (*types.ResourceCredential, error) {
	fullBody := types.VCenterCredentialWrapper{
		VCenterCredential: body,
	}
	defer TimeSpent("CreateVCenterResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential")

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPost, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// ModifyVCenterResourceCredential creates a new Resource Credential
func (s *System) ModifyVCenterResourceCredential(body types.VCenterCredential, id string) (*types.ResourceCredential, error) {
	fullBody := types.VCenterCredentialWrapper{
		VCenterCredential: body,
	}
	defer TimeSpent("ModifyVCenterResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential/%v", id)

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPut, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// CreateElementManagerResourceCredential creates a new Resource Credential
func (s *System) CreateElementManagerResourceCredential(body types.EMCredential) (*types.ResourceCredential, error) {
	// Validations
	// Set to default SNMPv2CommunityString if empty, set the SNMPv2Protocol to "SSH"
	if body.SNMPv2CommunityString == "" {
		body.SNMPv2CommunityString = "public"
	}
	body.SNMPv2Protocol = "SSH"

	fullBody := types.ElementManagerCredentialWrapper{
		EMCredential: body,
	}
	defer TimeSpent("CreateElementManagerResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential")

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPost, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// ModifyElementManagerResourceCredential creates a new Resource Credential
func (s *System) ModifyElementManagerResourceCredential(body types.EMCredential, id string) (*types.ResourceCredential, error) {
	// Validations
	// Set to default SNMPv2CommunityString if empty, set the SNMPv2Protocol to "SSH"
	if body.SNMPv2CommunityString == "" {
		body.SNMPv2CommunityString = "public"
	}
	body.SNMPv2Protocol = "SSH"

	fullBody := types.ElementManagerCredentialWrapper{
		EMCredential: body,
	}
	defer TimeSpent("ModifyElementManagerResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential/%v", id)

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPut, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// CreateScaleIOResourceCredential creates a new Resource Credential
func (s *System) CreateScaleIOResourceCredential(body types.ScaleIOCredential) (*types.ResourceCredential, error) {
	fullBody := types.GatewayCredentialWrapper{
		ScaleIOCredential: body,
	}
	defer TimeSpent("CreateScaleIOResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential")

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPost, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// ModifyScaleIOResourceCredential creates a new Resource Credential
func (s *System) ModifyScaleIOResourceCredential(body types.ScaleIOCredential, id string) (*types.ResourceCredential, error) {
	fullBody := types.GatewayCredentialWrapper{
		ScaleIOCredential: body,
	}
	defer TimeSpent("ModifyScaleIOResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential/%v", id)

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPut, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// CreatePresentationServerResourceCredential creates a new Resource Credential
func (s *System) CreatePresentationServerResourceCredential(body types.PSCredential) (*types.ResourceCredential, error) {
	fullBody := types.PresentationServerCredentialWrapper{
		PSCredential: body,
	}
	defer TimeSpent("CreatePresentationServerResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential")

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPost, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// ModifyPresentationServerResourceCredential creates a new Resource Credential
func (s *System) ModifyPresentationServerResourceCredential(body types.PSCredential, id string) (*types.ResourceCredential, error) {
	fullBody := types.PresentationServerCredentialWrapper{
		PSCredential: body,
	}
	defer TimeSpent("ModifyPresentationServerResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential/%v", id)

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPut, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// CreateOsAdminResourceCredential creates a new Resource Credential
func (s *System) CreateOsAdminResourceCredential(body types.OSAdminCredential) (*types.ResourceCredential, error) {
	// Validations
	if (body.SSHPrivateKey != "" && body.KeyPairName == "") || (body.SSHPrivateKey == "" && body.KeyPairName != "") {
		return nil, fmt.Errorf("If using an SSHPrivateKey then both KeyPairName and SSHPrivateKey must be set")
	}
	// For Admin the username is defaulted "root"
	if body.Username == "" {
		body.Username = "root"
	}
	fullBody := types.OsAdminCredentialWrapper{
		OSAdminCredential: body,
	}
	defer TimeSpent("CreateOsAdminResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential")

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPost, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// ModifyOsAdminResourceCredential creates a new Resource Credential
func (s *System) ModifyOsAdminResourceCredential(body types.OSAdminCredential, id string) (*types.ResourceCredential, error) {
	// Validations
	if (body.SSHPrivateKey != "" && body.KeyPairName == "") || (body.SSHPrivateKey == "" && body.KeyPairName != "") {
		return nil, fmt.Errorf("If using an SSHPrivateKey then both KeyPairName and SSHPrivateKey must be set")
	}
	// For Admin the username is defaulted "root"
	if body.Username == "" {
		body.Username = "root"
	}

	fullBody := types.OsAdminCredentialWrapper{
		OSAdminCredential: body,
	}
	defer TimeSpent("ModifyOsAdminResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential/%v", id)

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPut, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// CreateOsUserResourceCredential creates a new Resource Credential
func (s *System) CreateOsUserResourceCredential(body types.OSUserCredential) (*types.ResourceCredential, error) {
	// Validations
	if (body.SSHPrivateKey != "" && body.KeyPairName == "") || (body.SSHPrivateKey == "" && body.KeyPairName != "") {
		return nil, fmt.Errorf("If using an SSHPrivateKey then both KeyPairName and SSHPrivateKey must be set")
	}
	fullBody := types.OsUserCredentialWrapper{
		OSUserCredential: body,
	}
	defer TimeSpent("CreateOsUserResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential")

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPost, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

// ModifyOsUserResourceCredential creates a new Resource Credential
func (s *System) ModifyOsUserResourceCredential(body types.OSUserCredential, id string) (*types.ResourceCredential, error) {
	// Validations
	if (body.SSHPrivateKey != "" && body.KeyPairName == "") || (body.SSHPrivateKey == "" && body.KeyPairName != "") {
		return nil, fmt.Errorf("If using an SSHPrivateKey then both KeyPairName and SSHPrivateKey must be set")
	}
	fullBody := types.OsUserCredentialWrapper{
		OSUserCredential: body,
	}
	defer TimeSpent("ModifyOsUserResourceCredential", time.Now())

	path := fmt.Sprintf("/api/v1/Credential/%v", id)

	var credentialResult types.ResourceCredential
	_, err := s.client.xmlRequest(http.MethodPut, path, fullBody, &credentialResult)
	if err != nil {
		return nil, err
	}

	return &credentialResult, nil
}

func (s *System) DeleteResourceCredential(id string) error {
	path := fmt.Sprintf(
		"/api/v1/Credential/%v", id)
	param := &types.EmptyPayload{}
	err := s.client.getJSONWithRetry(
		http.MethodDelete, path, param, nil)
	if err != nil {
		return err
	}
	return nil
}
