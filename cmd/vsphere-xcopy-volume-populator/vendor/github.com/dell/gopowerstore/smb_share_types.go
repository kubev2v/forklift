/*
 *
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

package gopowerstore

// SMBShareCreate defines struct for creating SMB share
type SMBShareCreate struct {
	FileSystemID                    string `json:"file_system_id"`
	Name                            string `json:"name"`
	Path                            string `json:"path"`
	Description                     string `json:"description,omitempty"`
	IsContinuousAvailabilityEnabled bool   `json:"is_continuous_availability_enabled,omitempty"`
	IsEncryptionEnabled             bool   `json:"is_encryption_enabled,omitempty"`
	IsABEEnabled                    bool   `json:"is_ABE_enabled,omitempty"`
	IsBranchCacheEnabled            bool   `json:"is_branch_cache_enabled,omitempty"`
	OfflineAvailability             string `json:"offline_availability,omitempty"`
	Umask                           string `json:"umask,omitempty"`
}

// SMBShareModify defines struct for modifying SMB share
type SMBShareModify struct {
	Description                     *string `json:"description,omitempty"`
	IsContinuousAvailabilityEnabled *bool   `json:"is_continuous_availability_enabled,omitempty"`
	IsEncryptionEnabled             *bool   `json:"is_encryption_enabled,omitempty"`
	IsABEEnabled                    *bool   `json:"is_ABE_enabled,omitempty"`
	IsBranchCacheEnabled            *bool   `json:"is_branch_cache_enabled,omitempty"`
	OfflineAvailability             string  `json:"offline_availability,omitempty"`
	Umask                           string  `json:"umask,omitempty"`
}

// SMBShare details about a SMB Share
type SMBShare struct {
	ID                              string `json:"id"`
	FileSystemID                    string `json:"file_system_id"`
	Name                            string `json:"name"`
	Path                            string `json:"path"`
	Description                     string `json:"description"`
	IsContinuousAvailabilityEnabled bool   `json:"is_continuous_availability_enabled"`
	IsEncryptionEnabled             bool   `json:"is_encryption_enabled"`
	IsABEEnabled                    bool   `json:"is_ABE_enabled"`
	IsBranchCacheEnabled            bool   `json:"is_branch_cache_enabled"`
	OfflineAvailability             string `json:"offline_availability"`
	Umask                           string `json:"umask"`
	OfflineAvailabilityL10N         string `json:"offline_availability_l10n"`
}

func (share *SMBShare) Fields() []string {
	return []string{"id", "name", "file_system_id", "path", "description", "is_continuous_availability_enabled", "is_encryption_enabled", "is_ABE_enabled", "is_branch_cache_enabled", "offline_availability", "umask", "offline_availability_l10n"}
}

// SMBShareAce defines struct for SMB ACE
type SMBShareAce struct {
	TrusteeType string `json:"trustee_type"`
	TrusteeName string `json:"trustee_name"`
	AccessLevel string `json:"access_level"`
	AccessType  string `json:"access_type"`
}

// ModifySMBShareACL defines struct for modifying SMB ACL
type ModifySMBShareACL struct {
	Aces       []SMBShareAce `json:"aces,omitempty"`
	AddAces    []SMBShareAce `json:"add_aces,omitempty"`
	RemoveAces []SMBShareAce `json:"remove_aces,omitempty"`
}

// SMBShareACL  defines struct for SMB ACL
type SMBShareACL struct {
	Aces []SMBShareAce `json:"aces"`
}
