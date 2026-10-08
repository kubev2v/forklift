// Package blockdev discovers host block devices that are candidates for export
// over NBD: additional disks that are not the root disk and are not in use by the
// host (no mountpoints anywhere in their subtree).
package blockdev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Device is a discovered block device eligible for export.
type Device struct {
	// WWID is a stable identifier for the disk (SCSI/NVMe WWN, or a serial-based
	// fallback). Used to label the container so re-runs are idempotent.
	WWID string
	Path string // e.g. /dev/sdb
	Size uint64 // bytes
	// ScsiAddr is the H:C:T:L address used to order exports deterministically.
	ScsiAddr string
}

type lsblkNode struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	RO         bool        `json:"ro"`
	Size       uint64      `json:"size"`
	MountPoint string      `json:"mountpoint"`
	Children   []lsblkNode `json:"children"`
}

// Discover runs lsblk + udevadm and returns disks that can be exported over NBD.
func Discover(ctx context.Context) ([]Device, error) {
	out, err := exec.CommandContext(ctx, "lsblk", "-J", "-b",
		"-o", "NAME,TYPE,RO,SIZE,MOUNTPOINT").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(bytes.TrimSpace(ee.Stderr)) > 0 {
			return nil, fmt.Errorf("running lsblk: %w: %s", err, bytes.TrimSpace(ee.Stderr))
		}
		return nil, fmt.Errorf("running lsblk: %w", err)
	}

	var parsed struct {
		BlockDevices []lsblkNode `json:"blockdevices"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("decoding lsblk output: %w", err)
	}

	var devices []Device
	for _, disk := range parsed.BlockDevices {
		// Writable physical disks only; skip root/swap/data disks that are in use.
		if disk.Type != "disk" || disk.RO {
			continue
		}
		if strings.HasPrefix(disk.Name, "zram") ||
			strings.HasPrefix(disk.Name, "loop") ||
			strings.HasPrefix(disk.Name, "sr") {
			continue
		}
		if mountedAnywhere(disk) {
			continue
		}

		path := "/dev/" + disk.Name
		devices = append(devices, Device{
			WWID:     deviceID(ctx, path),
			Path:     path,
			Size:     disk.Size,
			ScsiAddr: scsiAddress(path),
		})
	}

	slices.SortFunc(devices, func(a, b Device) int {
		if c := strings.Compare(a.ScsiAddr, b.ScsiAddr); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	return devices, nil
}

// mountedAnywhere is true if this node or any child has a mountpoint set.
func mountedAnywhere(n lsblkNode) bool {
	if n.MountPoint != "" {
		return true
	}
	for _, c := range n.Children {
		if mountedAnywhere(c) {
			return true
		}
	}
	return false
}

// deviceID returns a stable disk id: scsi_id (matches VMware UUID when EnableUUID
// is on), else ID_WWN / ID_SERIAL from udev, else the device path.
func deviceID(ctx context.Context, path string) string {
	for _, bin := range []string{"/usr/lib/udev/scsi_id", "/lib/udev/scsi_id"} {
		out, err := exec.CommandContext(ctx, bin,
			"--whitelisted", "--replace-whitespace", "--device="+path).Output()
		if err == nil {
			if id := strings.TrimSpace(string(out)); id != "" {
				return id
			}
		}
	}

	out, err := exec.CommandContext(ctx, "udevadm", "info",
		"--query=property", "--name", path).Output()
	if err != nil {
		return path
	}
	props := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		key, val, ok := strings.Cut(line, "=")
		if ok {
			props[key] = val
		}
	}
	for _, key := range []string{"ID_WWN", "ID_SERIAL", "ID_SERIAL_SHORT"} {
		if v := strings.TrimSpace(props[key]); v != "" {
			return v
		}
	}
	return path
}

func scsiAddress(path string) string {
	name := strings.TrimPrefix(path, "/dev/")
	entries, err := os.ReadDir(filepath.Join("/sys/block", name, "device", "scsi_disk"))
	if err != nil || len(entries) == 0 {
		return path
	}
	return entries[0].Name()
}
