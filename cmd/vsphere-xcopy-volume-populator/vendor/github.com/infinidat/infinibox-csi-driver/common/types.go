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

import "log/slog"

// storage class parameter keys
const (
	LevelTrace  = slog.Level(-8)
	NodeRootDir = "/host"

	FSTypeExt3 = "ext3"
	FSTypeExt4 = "ext4"
	FSTypeXFS  = "xfs"

	StorageClassNFSExportPermissions = "nfs_export_permissions"
	StorageClassPrivPorts            = "privileged_ports_only"
	StorageClassSnapDirVisible       = "snapdir_visible"
	StorageClassUID                  = "uid"
	StorageClassGID                  = "gid"
	StorageClassUNIXPermissions      = "unix_permissions"
	StorageClassRoundup              = "round_up_requested_size"

	StorageClassSSDEnabled      = "ssd_enabled"
	StorageClassProvisionType   = "provision_type"
	StorageClassPoolName        = "pool_name"
	StorageClassNetworkSpace    = "network_space"
	StorageClassStorageProtocol = "storage_protocol"
	StorageClassFSPrefix        = "fs_prefix"
	StorageClassFSPrefixDefault = "csit_"
	StorageClassMaxVolsPerHost  = "max_vols_per_host"
	StorageClassUseCHAP         = "useCHAP"
	StorageClassThinProvision   = "THIN"
	StorageClassThickProvision  = "THICK"

	// ibox namespace services - indicate what protocol is available for a namespace.
	NetworkSpaceNFSService         = "NAS_SERVICE"
	NetworkSpaceISCSIService       = "ISCSI_SERVICE"
	NetworkSpaceReplicationService = "RMR_SERVICE"
	NetworkSpaceNVMEService        = "SAN_SERVICE"

	StorageClassMaxTreeqsPerFS    = "max_treeqs_per_filesystem"
	StorageClassMaxFilesystems    = "max_filesystems"
	StorageClassMaxFilesystemSize = "max_filesystem_size"

	StorageClassProtocolSecretAutoOrder = "preferred_auto_order" // a comma separated list of protocols
)

// storage protocols
const (
	ProtocolNFS   = "nfs"
	ProtocolTreeq = "nfs_treeq"
	ProtocolISCSI = "iscsi"
	ProtocolFC    = "fc"
	ProtocolNVME  = "nvme"
	ProtocolAuto  = "auto" // either fc, iscsi, or nvme - heuristically determined
)

// Service name in
const (
	ServiceName              = "infinibox-csi-driver"
	IBOXDefaultQueryPageSize = 1000
)

const LockExpiresAtParameter = "lock_expires_at"
const LockedState = "LOCKED"

// PVC annotations
const (
	PVCAnnotationPoolName     = "infinidat.com/pool_name"
	PVCAnnotationNetworkSpace = "infinidat.com/network_space"
	PVCAnnotationIBOXSecret   = "infinidat.com/ibox_secret"
)

const BytesInOneGibibyte = 1073741824

// for iscsi and fc host metadata
const CSICreatedHost = "csi-created-host"

// iboxreplica controller
const (
	PVCAnnotationSecretName      = "infinidat.com/secret_name"
	PVCAnnotationSecretNamespace = "infinidat.com/secret_namespace"

	ReplicaEntityCG          = "CONSISTENCY_GROUP"
	ReplicaEntityVolume      = "VOLUME"
	ReplicaEntityFilesystem  = "FILESYSTEM"
	ReplicationTypeASYNC     = "ASYNC"
	ReplicationBaseActionNew = "NEW"
)

const (
	CSIFSType                           = "csi.storage.k8s.io/fstype"
	CSIProvisionerSecretName            = "csi.storage.k8s.io/provisioner-secret-name"
	CSIProvisionerSecretNamespace       = "csi.storage.k8s.io/provisioner-secret-namespace"
	CSIControllerPublishSecretName      = "csi.storage.k8s.io/controller-publish-secret-name"
	CSIControllerPublishSecretNamespace = "csi.storage.k8s.io/controller-publish-secret-namespace"
	CSINodeStageSecretName              = "csi.storage.k8s.io/node-stage-secret-name"
	CSINodeStageSecretNamespace         = "csi.storage.k8s.io/node-stage-secret-namespace"
	CSINodePublishSecretName            = "csi.storage.k8s.io/node-publish-secret-name"
	CSINodePublishSecretNamespace       = "csi.storage.k8s.io/node-publish-secret-namespace"
	CSIControllerExpandSecretName       = "csi.storage.k8s.io/controller-expand-secret-name"
	CSIControllerExpandSecretNamespace  = "csi.storage.k8s.io/controller-expand-secret-namespace"
	CSINodeExpandSecretName             = "csi.storage.k8s.io/node-expand-secret-name"
	CSINodeExpandSecretNamespace        = "csi.storage.k8s.io/node-expand-secret-namespace"
	CSISnapshotterSecretName            = "csi.storage.k8s.io/snapshotter-secret-name"
)

const (
	EnvVarCSIDriverVersion = "CSI_DRIVER_VERSION"
	EnvVarCreateEvents     = "CREATE_EVENTS"
	EnvVarOSVersion        = "OS_VERSION"
	EnvVarKubeVersion      = "KUBE_VERSION"
	EnvVarNodeCount        = "NODE_COUNT"

	EnvVarProtocolSecret   = "PROTOCOL_SECRET"
	EnvVarPodNamespace     = "POD_NAMESPACE"
	EnvVarCleanupNFSPerms  = "CLEANUP_NFS_PERMS"
	EnvVarKubeNodeName     = "KUBE_NODE_NAME"
	EnvVarNodeIP           = "NODE_IP"
	EnvVarRemoveDomainName = "REMOVE_DOMAIN_NAME"
)

const (
	CustomEventCapacity   = "capacity"
	CustomEventVolumeCaps = "volume_caps"
	CustomEventNFSVersion = "nfs_version"
	CusstomEventVolumeID  = "volume_id"
	CustomEventVolumeName = "volume_name"
	CustomEventAction     = "csi_action"
)

const (
	CredentialHostname = "hostname"
	CredentialUsername = "username"
	CredentialPassword = "password"
)

const (
	IboxreplicaReplicaTypeSYNC          = "SYNC"
	IboxreplicaReplicaTypeASYNC         = "ASYNC"
	IboxreplicaReplicaTypeACTIVE_ACTIVE = "ACTIVE_ACTIVE"
)
