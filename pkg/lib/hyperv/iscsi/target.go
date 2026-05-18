package iscsi

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
	ps "github.com/kubev2v/forklift/pkg/lib/hyperv/powershell"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var log = logging.WithName("hyperv|iscsi")

// ErrTargetNotFound is returned when a requested iSCSI target does not exist.
var ErrTargetNotFound = errors.New("iSCSI target not found")

// ErrInvalidTargetName is returned when a target name contains invalid characters.
var ErrInvalidTargetName = errors.New("target name contains invalid characters")

// Alphanumeric, hyphens, dots only — prevents PowerShell injection in single-quoted literals.
var targetNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.\-]{0,222}$`)

func validateTargetName(name string) error {
	if !targetNameRe.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidTargetName, name)
	}
	return nil
}

// TargetClient manages iSCSI Target Server resources on a single Hyper-V host.
type TargetClient struct {
	drv driver.HyperVDriver
}

// NewTargetClient wraps an already-connected HyperVDriver.
// Panics if drv is nil (programmer error).
func NewTargetClient(drv driver.HyperVDriver) *TargetClient {
	if drv == nil {
		panic("NewTargetClient: driver cannot be nil")
	}
	return &TargetClient{drv: drv}
}

// Readiness holds the result of the host-level iSCSI prerequisite check.
type Readiness struct {
	FeatureInstalled bool
	FirewallOpen     bool
}

func (r *Readiness) Ready() bool {
	return r.FeatureInstalled && r.FirewallOpen
}

// TargetInfo is returned when querying an existing iSCSI target.
type TargetInfo struct {
	TargetIQN string `json:"TargetIqn"`
	Status    string `json:"Status"`
	LunCount  int    `json:"LunCount"`
}

// CreateTargetResult is returned after creating (or finding) an iSCSI target.
type CreateTargetResult struct {
	TargetIQN    string `json:"TargetIqn"`
	Created      bool   `json:"Created"`
	InitiatorIds string `json:"InitiatorIds,omitempty"`
}

// VirtualDiskResult is returned after creating a differencing iSCSI virtual disk.
type VirtualDiskResult struct {
	DevicePath string `json:"DevicePath"`
}

// LunMapping describes a single LUN inside a target.
type LunMapping struct {
	Path string `json:"Path"`
	Lun  int    `json:"Lun"`
}

// CheckReadiness verifies that the iSCSI Target Server feature is installed
// and TCP 3260 is reachable on the host. Both checks are read-only.
func (c *TargetClient) CheckReadiness() (*Readiness, error) {
	result := &Readiness{}

	featureOut, err := c.drv.ExecuteCommand(ps.CheckIscsiTargetFeature)
	if err != nil {
		return result, fmt.Errorf("iSCSI feature check failed: %w", err)
	}
	var feat struct {
		Installed bool `json:"Installed"`
	}
	if err := json.Unmarshal([]byte(featureOut), &feat); err != nil {
		return result, fmt.Errorf("parse iSCSI feature check response: %w (output: %s)", err, featureOut)
	}
	result.FeatureInstalled = feat.Installed

	if !result.FeatureInstalled {
		return result, nil
	}

	portOut, err := c.drv.ExecuteCommand(ps.CheckIscsiFirewallPort)
	if err != nil {
		return result, fmt.Errorf("iSCSI firewall port check failed: %w", err)
	}
	var port struct {
		Open bool `json:"Open"`
	}
	if err := json.Unmarshal([]byte(portOut), &port); err != nil {
		return result, fmt.Errorf("parse iSCSI firewall check response: %w (output: %s)", err, portOut)
	}
	result.FirewallOpen = port.Open

	return result, nil
}

// CreateTarget creates an iSCSI Server Target with an IQN-based initiator ACL.
func (c *TargetClient) CreateTarget(targetName, initiatorIQN string) (*CreateTargetResult, error) {
	if targetName == "" {
		return nil, fmt.Errorf("create iSCSI target: target name cannot be empty")
	}
	if err := validateTargetName(targetName); err != nil {
		return nil, fmt.Errorf("create iSCSI target: %w", err)
	}
	if initiatorIQN == "" {
		return nil, fmt.Errorf("create iSCSI target: initiator IQN cannot be empty")
	}
	cmd := ps.BuildCommand(ps.CreateIscsiTarget, targetName, initiatorIQN)
	stdout, err := c.drv.ExecuteCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("create iSCSI target %q: %w", targetName, err)
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil, fmt.Errorf("create iSCSI target %q: empty response", targetName)
	}
	var result CreateTargetResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return nil, fmt.Errorf("create iSCSI target %q: parse response: %w (output: %s)", targetName, err, stdout)
	}
	return &result, nil
}

// GetTarget retrieves information about an existing target.
// Returns ErrTargetNotFound if the target does not exist.
func (c *TargetClient) GetTarget(targetName string) (*TargetInfo, error) {
	if err := validateTargetName(targetName); err != nil {
		return nil, fmt.Errorf("get iSCSI target: %w", err)
	}
	cmd := ps.BuildCommand(ps.GetIscsiTarget, targetName)
	stdout, err := c.drv.ExecuteCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("get iSCSI target %q: %w", targetName, err)
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil, ErrTargetNotFound
	}
	var info TargetInfo
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		return nil, fmt.Errorf("get iSCSI target %q: parse response: %w (output: %s)", targetName, err, stdout)
	}
	return &info, nil
}

// RemoveTarget removes an iSCSI target, its disk mappings, and virtual disks.
func (c *TargetClient) RemoveTarget(targetName string) error {
	if err := validateTargetName(targetName); err != nil {
		return fmt.Errorf("remove iSCSI target: %w", err)
	}

	// If the target doesn't exist, nothing to do.
	info, err := c.GetTarget(targetName)
	if errors.Is(err, ErrTargetNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove iSCSI target %q: check existence: %w", targetName, err)
	}

	// Unmap and remove all virtual disks associated with the target.
	if info.LunCount > 0 {
		mappings, err := c.ListLunMappings(targetName)
		if err != nil {
			log.Error(err, "failed to list LUN mappings during target removal", "target", targetName)
		}
		for _, m := range mappings {
			if unmapErr := c.UnmapDiskFromTarget(targetName, m.Path); unmapErr != nil {
				log.Error(unmapErr, "failed to unmap disk during target removal", "target", targetName, "path", m.Path)
			}
			if rmErr := c.RemoveVirtualDisk(m.Path); rmErr != nil {
				log.Error(rmErr, "failed to remove virtual disk during target removal", "path", m.Path)
			}
		}
	}

	// Remove the target itself with retry.
	const maxRetries = 3
	removeCmd := ps.BuildCommand(ps.RemoveIscsiServerTarget, targetName)
	for i := 0; i < maxRetries; i++ {
		_, _ = c.drv.ExecuteCommand(removeCmd)
		if _, checkErr := c.GetTarget(targetName); errors.Is(checkErr, ErrTargetNotFound) {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("failed to remove iSCSI target %q after %d attempts", targetName, maxRetries)
}

// EnsureTargetDir creates the staging directory for differencing disks.
func (c *TargetClient) EnsureTargetDir() error {
	cmd := ps.BuildCommand(ps.EnsureIscsiTargetDir, ps.IscsiTargetDir)
	_, err := c.drv.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("ensure iSCSI target directory: %w", err)
	}
	return nil
}

// CreateVirtualDisk creates a differencing disk referencing parentVhdxPath and
// registers it as an iSCSI virtual disk. If a stale disk exists with a different
// parent, it is removed first. The diffDiskPath is typically obtained from
// powershell.DiffDiskPath().
func (c *TargetClient) CreateVirtualDisk(diffDiskPath, parentVhdxPath string) (*VirtualDiskResult, error) {
	// Check if a virtual disk already exists at this path.
	checkCmd := ps.BuildCommand(ps.GetIscsiVirtualDisk, diffDiskPath)
	checkOut, err := c.drv.ExecuteCommand(checkCmd)
	if err != nil {
		return nil, fmt.Errorf("create iSCSI virtual disk %q: check existing: %w", diffDiskPath, err)
	}
	checkOut = strings.TrimSpace(checkOut)

	if checkOut != "" {
		var existing struct {
			Path string `json:"Path"`
		}
		if err := json.Unmarshal([]byte(checkOut), &existing); err == nil && existing.Path != "" {
			// Verify the parent path matches; if not, remove the stale disk.
			parentCmd := ps.BuildCommand(ps.GetVHDParentPath, diffDiskPath)
			parentOut, _ := c.drv.ExecuteCommand(parentCmd)
			if strings.TrimSpace(parentOut) == parentVhdxPath {
				return &VirtualDiskResult{DevicePath: existing.Path}, nil
			}
			// Stale disk — remove and recreate.
			if rmErr := c.RemoveVirtualDisk(diffDiskPath); rmErr != nil {
				log.Error(rmErr, "failed to remove stale virtual disk", "path", diffDiskPath)
			}
			rmFileCmd := ps.BuildCommand(ps.RemoveFileByPath, diffDiskPath, diffDiskPath)
			_, _ = c.drv.ExecuteCommand(rmFileCmd)
		}
	}

	// Create the new differencing disk.
	createCmd := ps.BuildCommand(ps.NewIscsiVirtualDisk, diffDiskPath, parentVhdxPath)
	stdout, err := c.drv.ExecuteCommand(createCmd)
	if err != nil {
		return nil, fmt.Errorf("create iSCSI virtual disk %q (parent %q): %w", diffDiskPath, parentVhdxPath, err)
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil, fmt.Errorf("create iSCSI virtual disk %q: empty response", diffDiskPath)
	}
	var raw struct {
		Path string `json:"Path"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return nil, fmt.Errorf("create iSCSI virtual disk %q: parse response: %w (output: %s)", diffDiskPath, err, stdout)
	}
	return &VirtualDiskResult{DevicePath: raw.Path}, nil
}

// MapDiskToTarget maps a virtual disk to an iSCSI target at the given LUN ID.
func (c *TargetClient) MapDiskToTarget(targetName, diffDiskPath string, lunID int) error {
	cmd := ps.BuildCommand(ps.AddIscsiVirtualDiskTargetMapping, targetName, diffDiskPath, fmt.Sprintf("%d", lunID))
	_, err := c.drv.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("map disk %q to target %q LUN %d: %w", diffDiskPath, targetName, lunID, err)
	}
	return nil
}

// UnmapDiskFromTarget removes a single disk mapping from a target.
func (c *TargetClient) UnmapDiskFromTarget(targetName, diffDiskPath string) error {
	cmd := ps.BuildCommand(ps.RemoveIscsiVirtualDiskTargetMapping, targetName, diffDiskPath)
	_, err := c.drv.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("unmap disk %q from target %q: %w", diffDiskPath, targetName, err)
	}
	return nil
}

// RemoveVirtualDisk removes a single iSCSI virtual disk and its differencing disk file.
func (c *TargetClient) RemoveVirtualDisk(diffDiskPath string) error {
	cmd := ps.BuildCommand(ps.RemoveIscsiVirtualDisk, diffDiskPath)
	_, err := c.drv.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("remove iSCSI virtual disk %q: %w", diffDiskPath, err)
	}
	return nil
}

// CleanupDiffDisks removes all differencing disk mappings, virtual disks, and
// files for a VM from a target. The target itself is preserved for potential
// retry. Orchestrates existing primitives: list mappings → filter by pattern →
// unmap → remove virtual disk → delete leftover files.
func (c *TargetClient) CleanupDiffDisks(targetName, vmFilePattern string) error {
	mappings, err := c.ListLunMappings(targetName)
	if err != nil {
		log.Error(err, "failed to list LUN mappings during cleanup", "target", targetName)
	}

	for _, m := range mappings {
		if !pathMatchesPattern(m.Path, vmFilePattern) {
			continue
		}
		if unmapErr := c.UnmapDiskFromTarget(targetName, m.Path); unmapErr != nil {
			log.Error(unmapErr, "failed to unmap disk during cleanup", "target", targetName, "path", m.Path)
		}
		if rmErr := c.RemoveVirtualDisk(m.Path); rmErr != nil {
			log.Error(rmErr, "failed to remove virtual disk during cleanup", "path", m.Path)
		}
	}

	// Delete any leftover differencing disk files from the filesystem.
	rmCmd := ps.BuildCommand(ps.RemoveFilesByPattern, vmFilePattern)
	if _, err := c.drv.ExecuteCommand(rmCmd); err != nil {
		return fmt.Errorf("cleanup diff disk files for target %q (pattern %q): %w", targetName, vmFilePattern, err)
	}
	return nil
}

// pathMatchesPattern does a simple suffix-based match for patterns like
// "C:\iscsi-targets\forklift-abc123-*" against paths like
// "C:\iscsi-targets\forklift-abc123-disk0.vhdx".
func pathMatchesPattern(path, pattern string) bool {
	if !strings.HasSuffix(pattern, "*") {
		return strings.EqualFold(path, pattern)
	}
	prefix := pattern[:len(pattern)-1]
	return len(path) >= len(prefix) && strings.EqualFold(path[:len(prefix)], prefix)
}

// ListLunMappings returns all LUN mappings for a target.
func (c *TargetClient) ListLunMappings(targetName string) ([]LunMapping, error) {
	cmd := ps.BuildCommand(ps.GetIscsiVirtualDiskTargetMappings, targetName)
	stdout, err := c.drv.ExecuteCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("list LUN mappings for target %q: %w", targetName, err)
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil, nil
	}

	var mappings []LunMapping
	if err := json.Unmarshal([]byte(stdout), &mappings); err != nil {
		// PowerShell returns a bare object (not array) for a single mapping.
		var single LunMapping
		if err2 := json.Unmarshal([]byte(stdout), &single); err2 != nil {
			return nil, fmt.Errorf("list LUN mappings for target %q: parse as array: %v, as object: %v (output: %s)", targetName, err, err2, stdout)
		}
		mappings = append(mappings, single)
	}
	return mappings, nil
}

// SetupDiskForMigration is a method that performs the full
// differencing-disk workflow for a single VHDX.
// It returns the Windows path of the created differencing disk.
func (c *TargetClient) SetupDiskForMigration(targetName, parentVhdxPath string, diskIndex int) (string, error) {
	if err := validateTargetName(targetName); err != nil {
		return "", fmt.Errorf("setup disk for migration: %w", err)
	}
	if err := c.EnsureTargetDir(); err != nil {
		return "", err
	}

	diffPath := ps.DiffDiskPath(targetName, diskIndex)

	vd, err := c.CreateVirtualDisk(diffPath, parentVhdxPath)
	if err != nil {
		return "", err
	}

	if err := c.MapDiskToTarget(targetName, vd.DevicePath, diskIndex); err != nil {
		// Best-effort rollback of the virtual disk we just created.
		if cleanErr := c.RemoveVirtualDisk(diffPath); cleanErr != nil {
			log.Error(cleanErr, "rollback: failed to remove virtual disk after mapping failure",
				"diffDisk", diffPath)
		}
		return "", err
	}

	return vd.DevicePath, nil
}

// TeardownVM removes all differencing disks and the iSCSI target for a VM.
func (c *TargetClient) TeardownVM(targetName string) error {
	if err := validateTargetName(targetName); err != nil {
		return fmt.Errorf("teardown VM: %w", err)
	}
	pattern := ps.DiffDiskPattern(targetName)
	if err := c.CleanupDiffDisks(targetName, pattern); err != nil {
		log.Error(err, "failed to cleanup diff disks during teardown", "target", targetName)
	}

	if err := c.RemoveTarget(targetName); err != nil {
		return fmt.Errorf("teardown VM target %q: %w", targetName, err)
	}
	return nil
}
