/*
Copyright 2023 Infinidat
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
package common

// storage class parameter keys
const (
	NODE_ROOT_DIR = "/host"

	FS_TYPE_EXT3 = "ext3"
	FS_TYPE_EXT4 = "ext4"
	FS_TYPE_XFS  = "xfs"

	SC_NFS_EXPORT_PERMISSIONS = "nfs_export_permissions"
	SC_PRIV_PORTS             = "privileged_ports_only"
	SC_SNAPDIR_VISIBLE        = "snapdir_visible"
	SC_UID                    = "uid"
	SC_GID                    = "gid"
	SC_UNIX_PERMISSIONS       = "unix_permissions"
	SC_ROUND_UP               = "round_up_requested_size"

	SC_SSD_ENABLED          = "ssd_enabled"
	SC_PROVISION_TYPE       = "provision_type"
	SC_POOL_NAME            = "pool_name"
	SC_NETWORK_SPACE        = "network_space"
	SC_STORAGE_PROTOCOL     = "storage_protocol"
	SC_FS_PREFIX            = "fs_prefix"
	SC_FS_PREFIX_DEFAULT    = "csit_"
	SC_MAX_VOLS_PER_HOST    = "max_vols_per_host"
	SC_USE_CHAP             = "useCHAP"
	SC_THIN_PROVISION_TYPE  = "THIN"
	SC_THICK_PROVISION_TYPE = "THICK"

	// ibox namespace services - indicate what protocol is available for a namespace.
	NS_NFS_SVC         = "NAS_SERVICE"
	NS_ISCSI_SVC       = "ISCSI_SERVICE"
	NS_REPLICATION_SVC = "RMR_SERVICE"
	NS_NVME_SVC        = "SAN_SERVICE"

	SC_MAX_TREEQS_PER_FILESYSTEM = "max_treeqs_per_filesystem"
	SC_MAX_FILESYSTEMS           = "max_filesystems"
	SC_MAX_FILESYSTEM_SIZE       = "max_filesystem_size"
)

// storage protocols
const (
	PROTOCOL_NFS   = "nfs"
	PROTOCOL_TREEQ = "nfs_treeq"
	PROTOCOL_ISCSI = "iscsi"
	PROTOCOL_FC    = "fc"
	PROTOCOL_NVME  = "nvme"
)

// Service name in
const (
	SERVICE_NAME                 = "infinibox-csi-driver"
	IBOX_DEFAULT_QUERY_PAGE_SIZE = 1000
)

const LOCK_EXPIRES_AT_PARAMETER = "lock_expires_at"
const LOCKED_STATE = "LOCKED"

// PVC annotations
const (
	PVC_ANNOTATION_POOL_NAME     = "infinidat.com/pool_name"
	PVC_ANNOTATION_NETWORK_SPACE = "infinidat.com/network_space"
	PVC_ANNOTATION_IBOX_SECRET   = "infinidat.com/ibox_secret"
)

const BytesInOneGibibyte = 1073741824

// for iscsi and fc host metadata
const CSI_CREATED_HOST = "csi-created-host"

// iboxreplica controller
const (
	PVC_ANNOTATION_SECRET_NAME      = "infinidat.com/secret_name"
	PVC_ANNOTATION_SECRET_NAMESPACE = "infinidat.com/secret_namespace"

	REPLICA_ENTITY_CONSISTENCY_GROUP = "CONSISTENCY_GROUP"
	REPLICA_ENTITY_VOLUME            = "VOLUME"
	REPLICA_ENTITY_FILESYSTEM        = "FILESYSTEM"
	REPLICATION_TYPE_ASYNC           = "ASYNC"
	REPLICATION_BASE_ACTION_NEW      = "NEW"
)

const (
	SC_FSTYPE                         = "csi.storage.k8s.io/fstype"
	SC_PROVISIONER_SECRET_NAME        = "csi.storage.k8s.io/provisioner-secret-name"
	SC_CONTROLLER_PUBLISH_SECRET_NAME = "csi.storage.k8s.io/controller-publish-secret-name"
	SC_NODE_STAGE_SECRET_NAME         = "csi.storage.k8s.io/node-stage-secret-name"
	SC_NODE_PUBLISH_SECRET_NAME       = "csi.storage.k8s.io/node-publish-secret-name"
	SC_CONTROLLER_EXPAND_SECRET_NAME  = "csi.storage.k8s.io/controller-expand-secret-name"
	SC_NODE_EXPAND_SECRET_NAME        = "csi.storage.k8s.io/node-expand-secret-name"
	VOLUME_SNAPSHOT_CLASS_SECRET_NAME = "csi.storage.k8s.io/snapshotter-secret-name"
)

const (
	ENV_VAR_CSI_DRIVER_VERSION = "CSI_DRIVER_VERSION"
	ENV_VAR_CREATE_EVENTS      = "CREATE_EVENTS"
	ENV_VAR_OS_VERSION         = "OS_VERSION"
	ENV_VAR_KUBE_VERSION       = "KUBE_VERSION"
	ENV_VAR_NODE_COUNT         = "NODE_COUNT"
)

const (
	CUSTOM_EVENT_CAPACITY    = "capacity"
	CUSTOM_EVENT_VOLUME_CAPS = "volume_caps"
	CUSTOM_EVENT_NFS_VERSION = "nfs_version"
	CUSTOM_EVENT_VOLUME_ID   = "volume_id"
	CUSTOM_EVENT_VOLUME_NAME = "volume_name"
	CUSTOM_EVENT_ACTION      = "csi_action"
)

const (
	CRED_HOSTNAME = "hostname"
	CRED_USERNAME = "username"
	CRED_PASSWORD = "password"
)

const (
	IBOXREPLICA_REPLICA_TYPE_ASYNC         = "ASYNC"
	IBOXREPLICA_REPLICA_TYPE_ACTIVE_ACTIVE = "ACTIVE_ACTIVE"
)
