/*
 *
 * Copyright © 2021-2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package v100

// FileSystemIDName holds id and name for a file system
type FileSystemIDName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FileSystemList file system list resulted
type FileSystemList struct {
	FileSystemList []FileSystemIDName `json:"result"`
	From           int                `json:"from"`
	To             int                `json:"to"`
}

// FileSystemIterator holds the iterator of resultant file system list
type FileSystemIterator struct {
	ResultList     FileSystemList `json:"resultList"`
	ID             string         `json:"id"`
	Count          int            `json:"count"`
	ExpirationTime int64          `json:"expirationTime"`
	MaxPageSize    int            `json:"maxPageSize"`
}

// FileSystem holds information about a file system
type FileSystem struct {
	ID                        string       `json:"id"`
	ParentOID                 string       `json:"parent_oid"`
	Name                      string       `json:"name"`
	StorageWWN                string       `json:"storage_wwn"`
	ExportFSID                string       `json:"export_fsid"`
	Description               string       `json:"description"`
	SizeTotal                 int64        `json:"size_total"`
	SizeUsed                  int64        `json:"size_used"`
	Health                    Health       `json:"health"`
	ReadOnly                  bool         `json:"read_only"`
	FsType                    string       `json:"fs_type"`
	MountState                string       `json:"mount_state"`
	AccessPolicy              string       `json:"access_policy"`
	LockingPolicy             string       `json:"locking_policy"`
	FolderRenamePolicy        string       `json:"folder_rename_policy"`
	HostIOBlockSize           int          `json:"host_ioblock_size"`
	NasServer                 string       `json:"nas_server"`
	SmbSyncWrites             bool         `json:"smb_sync_writes"`
	SmbOpLocks                bool         `json:"smb_op_locks"`
	SmbNoNotify               bool         `json:"smb_no_notify"`
	SmbNotifyOnAccess         bool         `json:"smb_notify_on_access"`
	SmbNotifyOnWrite          bool         `json:"smb_notify_on_write"`
	SmbNotifyOnChangeDirDepth int          `json:"smb_notify_on_change_dir_depth"`
	AsyncMtime                bool         `json:"async_mtime"`
	FlrMode                   string       `json:"flr_mode"`
	FlrMinRet                 string       `json:"flr_min_ret"`
	FlrDefRet                 string       `json:"flr_def_ret"`
	FlrMaxRet                 string       `json:"flr_max_ret"`
	FlrAutoLock               bool         `json:"flr_auto_lock"`
	FlrAutoDelete             bool         `json:"flr_auto_delete"`
	FlrPolicyInterval         int          `json:"flr_policy_interval"`
	FlrEnabled                bool         `json:"flr_enabled"`
	FlrClockTime              string       `json:"flr_clock_time"`
	FlrMaxRetentionDate       string       `json:"flr_max_retention_date"`
	FlrHasProtectedFiles      bool         `json:"flr_has_protected_files"`
	QuotaConfig               *QuotaConfig `json:"quota_config"`
	EventNotifications        string       `json:"event_notifications"`
	InfoThreshold             int          `json:"info_threshold"`
	HighThreshold             int          `json:"high_threshold"`
	WarningThreshold          int          `json:"warning_threshold"`
	ServiceLevel              string       `json:"service_level"`
	DataReduction             bool         `json:"data_reduction"`
}

// CreateFileSystem has payload to create file system
type CreateFileSystem struct {
	Name                      string       `json:"name"`
	SizeTotal                 int64        `json:"size_total"`
	FsType                    string       `json:"fs_type,omitempty"`
	AccessPolicy              string       `json:"access_policy,omitempty"`
	LockingPolicy             string       `json:"locking_policy,omitempty"`
	FolderRenamePolicy        string       `json:"folder_rename_policy,omitempty"`
	HostIOBlock               int          `json:"host_ioblock,omitempty"`
	NasServer                 string       `json:"nas_server"`
	SmbSyncWrites             bool         `json:"smb_sync_writes,omitempty"`
	SmbOpLocks                bool         `json:"smb_op_locks,omitempty"`
	SmbNoNotify               bool         `json:"smb_no_notify,omitempty"`
	SmbNotifyOnAccess         bool         `json:"smb_notify_on_access,omitempty"`
	SmbNotifyOnWrite          bool         `json:"smb_notify_on_write,omitempty"`
	SmbNotifyOnChangeDirDepth int          `json:"smb_notify_on_change_dir_depth,omitempty"`
	AsyncMtime                bool         `json:"async_mtime,omitempty"`
	FlrMode                   string       `json:"flr_mode,omitempty"`
	FlrMinRet                 string       `json:"flr_min_ret,omitempty"`
	FlrDefRet                 string       `json:"flr_def_ret,omitempty"`
	FlrMaxRet                 string       `json:"flr_max_ret,omitempty"`
	FlrAutoLock               bool         `json:"flr_auto_lock,omitempty"`
	FlrAutoDelete             bool         `json:"flr_auto_delete,omitempty"`
	FlrPolicyInterval         int          `json:"flr_policy_interval,omitempty"`
	FlrEnabled                bool         `json:"flr_enabled,omitempty"`
	FlrClockTime              string       `json:"flr_clock_time,omitempty"`
	FlrMaxRetentionDate       string       `json:"flr_max_retention_date,omitempty"`
	FlrHasProtectedFiles      bool         `json:"flr_has_protected_files,omitempty"`
	QuotaConfig               *QuotaConfig `json:"quota_config,omitempty"`
	EventNotifications        string       `json:"event_notifications,omitempty"`
	InfoThreshold             int          `json:"info_threshold,omitempty"`
	HighThreshold             int          `json:"high_threshold,omitempty"`
	WarningThreshold          int          `json:"warning_threshold,omitempty"`
	ServiceLevel              string       `json:"service_level,omitempty"`
	DataReduction             bool         `json:"data_reduction,omitempty"`
}

// QuotaConfig defines quotas for file
type QuotaConfig struct {
	QuotaEnabled     bool `json:"quota_enabled,omitempty"`
	GracePeriod      int  `json:"grace_period,omitempty"`
	DefaultHardLimit int  `json:"default_hard_limit,omitempty"`
	DefaultSoftLimit int  `json:"default_soft_limit,omitempty"`
}

// ModifyFileSystem params to modifies a file system
type ModifyFileSystem struct {
	SizeTotal                 int64        `json:"size_total,omitempty"`
	AccessPolicy              string       `json:"access_policy,omitempty"`
	LockingPolicy             string       `json:"locking_policy,omitempty"`
	FolderRenamePolicy        string       `json:"folder_rename_policy,omitempty"`
	SmbSyncWrites             bool         `json:"smb_sync_writes,omitempty"`
	SmbOpLocks                bool         `json:"smb_op_locks,omitempty"`
	SmbNoNotify               bool         `json:"smb_no_notify,omitempty"`
	SmbNotifyOnAccess         bool         `json:"smb_notify_on_access,omitempty"`
	SmbNotifyOnWrite          bool         `json:"smb_notify_on_write,omitempty"`
	SmbNotifyOnChangeDirDepth int          `json:"smb_notify_on_change_dir_depth,omitempty"`
	AsyncMtime                bool         `json:"async_mtime,omitempty"`
	FlrMinRet                 string       `json:"flr_min_ret,omitempty"`
	FlrDefRet                 string       `json:"flr_def_ret,omitempty"`
	FlrMaxRet                 string       `json:"flr_max_ret,omitempty"`
	FlrAutoLock               bool         `json:"flr_auto_lock,omitempty"`
	FlrAutoDelete             bool         `json:"flr_auto_delete,omitempty"`
	FlrPolicyInterval         int          `json:"flr_policy_interval,omitempty"`
	FlrClockTime              string       `json:"flr_clock_time,omitempty"`
	FlrMaxRetentionDate       string       `json:"flr_max_retention_date,omitempty"`
	FlrHasProtectedFiles      bool         `json:"flr_has_protected_files,omitempty"`
	QuotaConfig               *QuotaConfig `json:"quota_config,omitempty"`
	EventNotifications        string       `json:"event_notifications,omitempty"`
	InfoThreshold             int          `json:"info_threshold,omitempty"`
	HighThreshold             int          `json:"high_threshold,omitempty"`
	WarningThreshold          int          `json:"warning_threshold,omitempty"`
	ServiceLevel              string       `json:"service_level,omitempty"`
	DataReduction             bool         `json:"data_reduction,omitempty"`
}

// NFSExportIDName holds id and name for a file system
type NFSExportIDName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NFSExportList NFS export list resulted
type NFSExportList struct {
	NFSExportList []NFSExportIDName `json:"result"`
	From          int               `json:"from"`
	To            int               `json:"to"`
}

// NFSExportIterator holds the iterator of resultant file system list
type NFSExportIterator struct {
	ResultList     NFSExportList `json:"resultList"`
	ID             string        `json:"id"`
	Count          int           `json:"count"`
	ExpirationTime int64         `json:"expirationTime"`
	MaxPageSize    int           `json:"maxPageSize"`
}

// ModifyNFSExport holds param for modification
type ModifyNFSExport struct {
	Name               string   `json:"name,omitempty"`
	Path               string   `json:"path,omitempty"`
	Description        string   `json:"description,omitempty"`
	DefaultAccess      string   `json:"default_access,omitempty"`
	MinSecurity        string   `json:"min_security,omitempty"`
	NFSOwnerUsername   bool     `json:"nfs_owner_username,omitempty"`
	NoAccessHosts      []string `json:"no_access_hosts,omitempty"`
	ReadOnlyHosts      []string `json:"read_only_hosts,omitempty"`
	ReadOnlyRootHosts  []string `json:"read_only_root_hosts,omitempty"`
	ReadWriteHosts     []string `json:"read_write_hosts,omitempty"`
	ReadWriteRootHosts []string `json:"read_write_root_hosts,omitempty"`
	AnonymousUID       int      `json:"anonymous_uid,omitempty"`
	AnonymousGID       int      `json:"anonymous_gid,omitempty"`
	NoSUID             bool     `json:"no_suid,omitempty"`
}

// CreateNFSExport holds param to create NFS export
type CreateNFSExport struct {
	StorageResource    string   `json:"storage_resource"`
	Name               string   `json:"name"`
	Path               string   `json:"path"`
	Description        string   `json:"description,omitempty"`
	DefaultAccess      string   `json:"default_access,omitempty"`
	MinSecurity        string   `json:"min_security,omitempty"`
	NFSOwnerUsername   bool     `json:"nfs_owner_username,omitempty"`
	NoAccessHosts      []string `json:"no_access_hosts,omitempty"`
	ReadOnlyHosts      []string `json:"read_only_hosts,omitempty"`
	ReadOnlyRootHosts  []string `json:"read_only_root_hosts,omitempty"`
	ReadWriteHosts     []string `json:"read_write_hosts,omitempty"`
	ReadWriteRootHosts []string `json:"read_write_root_hosts,omitempty"`
	AnonymousUID       int      `json:"anonymous_uid,omitempty"`
	AnonymousGID       int      `json:"anonymous_gid,omitempty"`
	NoSUID             bool     `json:"no_suid,omitempty"`
}

// NFSExport holds export nfs export details
type NFSExport struct {
	ID                 string   `json:"id"`
	Type               string   `json:"type"`
	Role               string   `json:"role"`
	Filesystem         string   `json:"filesystem"`
	Snap               string   `json:"snap"`
	NASServer          string   `json:"nas_server"`
	Name               string   `json:"name"`
	Path               string   `json:"path"`
	Description        string   `json:"description"`
	DefaultAccess      string   `json:"default_access"`
	MinSecurity        string   `json:"min_security"`
	NFSOwnerUsername   string   `json:"nfs_owner_username"`
	NoAccessHosts      []string `json:"no_access_hosts"`
	ReadOnlyHosts      []string `json:"read_only_hosts"`
	ReadOnlyRootHosts  []string `json:"read_only_root_hosts"`
	ReadWriteHosts     []string `json:"read_write_hosts"`
	ReadWriteRootHosts []string `json:"read_write_root_hosts"`
	AnonymousUID       int      `json:"anonymous_uid"`
	AnonymousGID       int      `json:"anonymous_gid"`
	NoSUID             bool     `json:"no_suid"`
}

// NASServerList holds nas server metadata items
type NASServerList struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NASServerIterator holds the iterator of resultant NAS server list
type NASServerIterator struct {
	Entries []NASServerList `json:"entries"`
}

// Health holds health info
type Health struct {
	HealthStatus string `json:"health_status"`
}

// NASServer holds nas server details
type NASServer struct {
	ID                          string   `json:"id"`
	Health                      Health   `json:"health"`
	Name                        string   `json:"name"`
	StorageResourcePool         string   `json:"storage_resource_pool"`
	OperationalStatus           string   `json:"operational_status"`
	PrimaryNode                 string   `json:"primary_node"`
	BackupNode                  string   `json:"backup_node"`
	Cluster                     string   `json:"cluster"`
	ProductionMode              bool     `json:"production_mode"`
	CurrentUnixDirectoryService string   `json:"current_unix_directory_service"`
	UsernameTranslation         bool     `json:"username_translation"`
	AutoUserMapping             bool     `json:"auto_user_mapping"`
	FileInterfaces              []string `json:"file_interfaces"`
	PreferredInterfaceSettings  struct {
		CurrentPreferredIPV4 string `json:"current_preferred_ip_v4"`
	} `json:"preferred_interface_settings"`
	NFSServer   string `json:"nfs_server"`
	RootFSWWN   string `json:"root_fs_wwn"`
	ConfigFSWWN string `json:"config_fs_wwn"`
}

// ModifyNASServer modifies nas server
type ModifyNASServer struct {
	Name                        string `json:"name,omitempty"`
	CurrentUnixDirectoryService string `json:"current_unix_directory_service,omitempty"`
	UsernameTranslation         bool   `json:"username_translation,omitempty"`
	AutoUserMapping             bool   `json:"auto_user_mapping,omitempty"`
}

// FileInterface holds file interface details
type FileInterface struct {
	ID         string `json:"id"`
	NasServer  string `json:"nas_server"`
	NetDevice  string `json:"net_device"`
	MacAddress string `json:"mac_address"`
	IPAddress  string `json:"ip_address"`
	Netmask    string `json:"netmask"`
	Gateway    string `json:"gateway"`
	VlanID     int    `json:"vlan_id"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	IsDisabled bool   `json:"is_disabled"`
	Override   bool   `json:"override"`
}

// NFSServerList holds nfs server metadata items
type NFSServerList struct {
	ID string `json:"id"`
}

// NFSServerIterator holds the iterator of resultant NFS server list
type NFSServerIterator struct {
	Entries []NFSServerList `json:"entries"`
}

// NFSServer holds nfs server details
type NFSServer struct {
	ID           string `json:"id"`
	NFSV3Enabled bool   `json:"nfsv3_enabled"`
	NFSV4Enabled bool   `json:"nfsv4_enabled"`
}
