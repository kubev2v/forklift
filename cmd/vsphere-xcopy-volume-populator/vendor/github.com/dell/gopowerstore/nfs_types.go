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

type NFSExportDefaultAccessEnum string

const (
	NoAccess     NFSExportDefaultAccessEnum = "No_Access"
	ReadOnly     NFSExportDefaultAccessEnum = "Read_Only"
	ReadWrite    NFSExportDefaultAccessEnum = "Read_Write"
	Root         NFSExportDefaultAccessEnum = "Root"
	ReadOnlyRoot NFSExportDefaultAccessEnum = "Read_Only_Root "
)

// NFSExportCreate details about creation of new NFS export
type NFSExportCreate struct {
	// NFS Export name
	Name string `json:"name"`
	// NFS Export description
	Description string `json:"description,omitempty"`
	// Unique identifier of the file system on which the NFS Export was created
	FileSystemID string `json:"file_system_id"`
	// Local path to a location within the file system.
	Path string `json:"path"`
	// Default access level for all hosts that can access the Export
	DefaultAccess NFSExportDefaultAccessEnum `json:"default_access,omitempty"`
	// NFS enforced security type for users accessing an NFS Export.
	MinSecurity string `json:"min_security,omitempty"`
	// Hosts with no access to the NFS export or its snapshots.
	NoAccessHosts []string `json:"no_access_hosts,omitempty"`
	// Hosts with read-only access to the NFS export and its snapshots.
	ReadOnlyHosts []string `json:"read_only_hosts,omitempty"`
	// Hosts with read-only and read-only for root user access to the NFS Export and its snapshots.
	ReadOnlyRootHosts []string `json:"read_only_root_hosts,omitempty"`
	// Hosts with read and write access to the NFS export and its snapshots.
	ReadWriteHosts []string `json:"read_write_hosts,omitempty"`
	// Hosts with read and write and read and write for root user access to the NFS Export and its snapshots.
	ReadWriteRootHosts []string `json:"read_write_root_hosts,omitempty"`
	// Specifies the user ID of the anonymous account.
	// Zero ID is a valid value, so omitempty should not be used.
	AnonymousUID int32 `json:"anonymous_UID"`
	// Specifies the group ID of the anonymous account.
	// Zero ID is a valid value, so omitempty should not be used.
	AnonymousGID int32 `json:"anonymous_GID"`
	// If set, do not allow access to set SUID. Otherwise, allow access.
	IsNoSUID bool `json:"is_no_SUID"`
	// (*Applies to NFS shares of VMware NFS storage resources.*) Default owner of the NFS Export associated with the datastore. Required if secure NFS enabled. For NFSv3 or NFSv4 without Kerberos, the default owner is root. Was added in version 3.0.0.0.
	NFSOwnerUsername string `json:"nfs_owner_username,omitempty"`
}

// NFSExportModify details about modification of exiting NFS export
type NFSExportModify struct {
	// An optional description for the host.
	// The description should not be more than 256 UTF-8 characters long and should not have any unprintable characters.
	Description string `json:"description,omitempty"`

	// Read-Write
	// Hosts to add to the current read_write_hosts list. Hosts can be entered by Hostname, IP addresses
	AddRWHosts []string `json:"add_read_write_hosts,omitempty"`
	// Hosts to remove from the current read_write_hosts list. Hosts can be entered by Hostname, IP addresses.
	RemoveRWHosts []string `json:"remove_read_write_hosts,omitempty"`

	// Read-Only
	// Hosts to add to the current read_only_hosts list. Hosts can be entered by Hostname, IP addresses
	AddROHosts []string `json:"add_read_only_hosts,omitempty"`
	// Hosts to remove from the current read_only_hosts list. Hosts can be entered by Hostname, IP addresses.
	RemoveROHosts []string `json:"remove_read_only_hosts,omitempty"`

	// Read-Write, allow Root
	// Hosts to add to the current read_write_root_hosts list. Hosts can be entered by Hostname, IP addresses
	AddRWRootHosts []string `json:"add_read_write_root_hosts,omitempty"`
	// Hosts to remove from the current read_write_root_hosts list. Hosts can be entered by Hostname, IP addresses.
	RemoveRWRootHosts []string `json:"remove_read_write_root_hosts,omitempty"`

	// Read-Only, allow Roots
	// Hosts to add to the current read_only_hosts list. Hosts can be entered by Hostname, IP addresses
	AddRORootHosts []string `json:"add_read_only_root_hosts,omitempty"`
	// Hosts to remove from the current read_only_hosts list. Hosts can be entered by Hostname, IP addresses.
	RemoveRORootHosts []string `json:"remove_read_only_root_hosts,omitempty"`

	// No-Access
	// Hosts to add to the current no_access_hosts list. Hosts can be entered by Hostname, IP addresses
	AddNoAccessHosts []string `json:"add_no_access_hosts,omitempty"`
	// Hosts to remove from the current no_access_hosts list. Hosts can be entered by Hostname, IP addresses
	RemoveNoAccessHosts []string `json:"remove_no_access_hosts,omitempty"`

	// Default access level for all hosts that can access the Export.
	DefaultAccess string `json:"default_access,omitempty"`
	// NFS enforced security type for users accessing an NFS Export.
	MinSecurity string `json:"min_security,omitempty"`
	// Hosts with no access to the NFS export or its snapshots.
	NoAccessHosts []string `json:"no_access_hosts,omitempty"`
	// Hosts with read-only access to the NFS export and its snapshots.
	ReadOnlyHosts []string `json:"read_only_hosts,omitempty"`
	// Hosts with read-only and read-only for root user access to the NFS Export and its snapshots.
	ReadOnlyRootHosts []string `json:"read_only_root_hosts,omitempty"`
	// Hosts with read and write access to the NFS export and its snapshots.
	ReadWriteHosts []string `json:"read_write_hosts,omitempty"`
	// Hosts with read and write and read and write for root user access to the NFS Export and its snapshots.
	ReadWriteRootHosts []string `json:"read_write_root_hosts,omitempty"`
	// Specifies the user ID of the anonymous account.
	AnonymousUID int32 `json:"anonymous_UID,omitempty"`
	// Specifies the group ID of the anonymous account.
	AnonymousGID int32 `json:"anonymous_GID,omitempty"`
	// If set, do not allow access to set SUID. Otherwise, allow access.
	IsNoSUID bool `json:"is_no_SUID"`
	// (*Applies to NFS shares of VMware NFS storage resources.*) Default owner of the NFS Export associated with the datastore. Required if secure NFS enabled. For NFSv3 or NFSv4 without Kerberos, the default owner is root. Was added in version 3.0.0.0.
	NFSOwnerUsername string `json:"nfs_owner_usernamestring,omitempty"`
}

// NFSServerCreate details about creation of new NFS server
type NFSServerCreate struct {
	// Unique identifier of the NAS server
	NasServerID string `json:"nas_server_id"`
	// The name that will be used by NFS clients to connect to this NFS server
	// This name is required when using secure NFS
	HostName string `json:"host_name,omitempty"`
	// Indicates whether NFSv3 is enabled on the NAS server. When enabled, NFS shares can be accessed with NFSv3
	IsNFSv3Enabled bool `json:"is_nfsv3_enabled,omitempty"`
	// Indicates whether NFSv4 is enabled on the NAS server. When enabled, NFS shares can be accessed with NFSv4
	IsNFSv4Enabled bool `json:"is_nfsv4_enabled,omitempty"`
	// Indicates whether secure NFS is enabled on the NFS server
	IsSecureEnabled bool `json:"is_secure_enabled,omitempty"`
}

// Details about the NFSExport.
type NFSExport struct {
	// Unique id of the NFS Export
	ID string `json:"id,omitempty"`
	// Unique identifier of the file system on which the NFS Export was created
	FileSystemID string `json:"file_system_id,omitempty"`
	// NFS Export name
	Name string `json:"name,omitempty"`
	// NFS Export description
	Description string `json:"description,omitempty"`
	// Default access level for all hosts that can access the Export
	// [ No_Access, Read_Only, Read_Write, Root, Read_Only_Root ]
	DefaultAccess NFSExportDefaultAccessEnum `json:"default_access"`
	// Local path to a location within the file system.
	// With NFS, each export must have a unique local path.
	Path string `json:"path,omitempty"`
	// Read-Write hosts
	RWHosts []string `json:"read_write_hosts,omitempty"`
	// Read-Only hosts
	ROHosts []string `json:"read_only_hosts,omitempty"`
	// Read-Write, allow Root hosts
	RWRootHosts []string `json:"read_write_root_hosts,omitempty"`
	// Read-Only, allow Roots hosts
	RORootHosts []string `json:"read_only_root_hosts,omitempty"`
	// NFS enforced security type for users accessing an NFS Export.
	// [ Sys, Kerberos, Kerberos_With_Integrity, Kerberos_With_Encryption ]
	MinSecurity string `json:"min_security,omitempty"`
	// (*Applies to NFS shares of VMware NFS storage resources.*)
	// Default owner of the NFS Export associated with the datastore.
	// Required if secure NFS enabled.
	NFSOwnerUsername string `json:"nfs_owner_username,omitempty"`
	// Hosts with no access to the NFS export or its snapshots.
	NoAccessHosts []string `json:"no_access_hosts,omitempty"`
	// Specifies the user ID of the anonymous account.
	AnonymousUID int32 `json:"anonymous_UID,omitempty"`
	// Specifies the group ID of the anonymous account.
	AnonymousGID int32 `json:"anonymous_GID,omitempty"`
	// If set, do not allow access to set SUID. Otherwise, allow access.
	IsNoSUID bool `json:"is_no_SUID"`
}

// Details about the file interface
type FileInterface struct {
	// Unique id of the file interface
	ID string `json:"id"`
	// Ip address of file interface
	IPAddress string `json:"ip_address"`
}

// Fields returns fields which must be requested to fill struct
func (n *NFSExport) Fields() []string {
	return []string{"description", "id", "name", "file_system_id", "default_access", "path", "read_only_hosts", "read_only_root_hosts", "read_write_hosts", "read_write_root_hosts", "min_security", "nfs_owner_username", "no_access_hosts", "anonymous_UID", "anonymous_GID", "is_no_SUID"}
}

// Fields returns fields which must be requested to fill struct
func (n *FileInterface) Fields() []string {
	return []string{"id", "ip_address"}
}
