package types

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// SnapshotDiskInfo contains VM moref, snapshot moref, disk paths, and compute resource path for inspection
// This is used by both vm_service (to retrieve the info) and inspection (to use it)
// Supports multiple disks - DiskPaths and BaseDiskPaths are arrays
type SnapshotDiskInfo struct {
	VMMoref             string
	SnapshotMoref       string
	DiskPaths           []string // Current disk paths (may include snapshot deltas)
	BaseDiskPaths       []string // Base disk paths (without snapshot deltas)
	ComputeResourcePath string   // Path to compute resource (host/cluster) for vpx:// URL (e.g., "/Datacenter/Cluster/host.example.com")
}

// Credentials holds vCenter access details
type Credentials struct {
	VCenterURL string
	Username   string
	Password   string
}

// CacheKey represents a unique identifier for a VM+snapshot pair
type CacheKey struct {
	VMMoref       string
	SnapshotMoref string
}

// String returns a string representation of the cache key
func (k CacheKey) String() string {
	return fmt.Sprintf("%s:%s", k.VMMoref, k.SnapshotMoref)
}

// Hash returns a hash of the cache key for use as a storage key
func (k CacheKey) Hash() string {
	h := sha256.New()
	h.Write([]byte(k.String()))
	return hex.EncodeToString(h.Sum(nil))
}

// DB defines the interface for persisting inspection data
// Callers must implement this interface to provide persistence
type DB interface {
	// GetVirtInspectorXML retrieves VirtInspector inspection data for a given cache key
	// Returns nil if not found
	GetVirtInspectorXML(ctx context.Context, key CacheKey) (*VirtInspectorXML, error)

	// SetVirtInspectorXML stores VirtInspector inspection data for a given cache key
	SetVirtInspectorXML(ctx context.Context, key CacheKey, data *VirtInspectorXML) error

	// GetVirtV2VInspectorXML retrieves VirtV2vInspector inspection data for a given cache key
	// Returns nil if not found
	GetVirtV2VInspectorXML(ctx context.Context, key CacheKey) (*VirtV2VInspectorXML, error)

	// SetVirtV2VInspectorXML stores VirtV2vInspector inspection data for a given cache key
	SetVirtV2VInspectorXML(ctx context.Context, key CacheKey, data *VirtV2VInspectorXML) error
}
