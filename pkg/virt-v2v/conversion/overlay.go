package conversion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Overlay struct {
	// Path to the qcow2 overlay file (e.g. /var/tmp/v2v/vm-sda.qcow2)
	Path string
	// Path to the original backing disk (e.g. /dev/block0 or /mnt/disks/disk0/disk.img)
	BackingPath string
	// The Disk whose Link symlink was rewired to point to the overlay
	Disk *Disk
	// Original symlink target before rewire
	OriginalLink string
}

func (c *Conversion) detectDiskFormat(path string) (string, error) {
	var stdout bytes.Buffer
	cmd := c.CommandBuilder.New("qemu-img").
		AddPositional("info").
		AddArg("--output", "json").
		AddPositional(path).
		Build()
	cmd.SetStdout(&stdout)
	cmd.SetStderr(os.Stderr)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("qemu-img info failed for %s: %w", path, err)
	}
	var info struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		return "", fmt.Errorf("failed to parse qemu-img info output for %s: %w", path, err)
	}
	if info.Format == "" {
		fmt.Fprintf(os.Stderr, "WARNING: qemu-img info returned empty format for %s, assuming raw\n", path)
		return "raw", nil
	}
	return info.Format, nil
}

func (c *Conversion) CreateOverlays() ([]*Overlay, error) {
	var overlays []*Overlay
	for _, disk := range c.Disks {
		overlayPath := disk.Link + ".qcow2"

		// Detect the backing disk format instead of assuming raw
		backingFmt, err := c.detectDiskFormat(disk.Path)
		if err != nil {
			c.rollbackOverlays(overlays)
			return nil, fmt.Errorf("failed to detect format for %s: %w", disk.Path, err)
		}

		// Step 1: Create qcow2 overlay file backed by the original disk
		cmd := c.CommandBuilder.New("qemu-img").
			AddPositional("create").
			AddArg("-f", "qcow2").
			AddArg("-b", disk.Path).
			AddArg("-F", backingFmt).
			AddPositional(overlayPath).
			Build()
		cmd.SetStdout(os.Stdout)
		cmd.SetStderr(os.Stderr)
		if err := cmd.Run(); err != nil {
			_ = os.Remove(overlayPath)
			c.rollbackOverlays(overlays)
			return nil, fmt.Errorf("failed to create overlay for %s: %w", disk.Path, err)
		}

		// Step 2: Read and remove old symlink (pointed at base disk)
		originalLink, err := os.Readlink(disk.Link)
		if err != nil {
			_ = os.Remove(overlayPath)
			c.rollbackOverlays(overlays)
			return nil, fmt.Errorf("failed to read symlink %s: %w", disk.Link, err)
		}
		if err := os.Remove(disk.Link); err != nil && !os.IsNotExist(err) {
			_ = os.Remove(overlayPath)
			c.rollbackOverlays(overlays)
			return nil, fmt.Errorf("failed to remove old symlink %s: %w", disk.Link, err)
		}

		// Step 3: Create new symlink pointing at the overlay
		if err := os.Symlink(overlayPath, disk.Link); err != nil {
			_ = os.Remove(overlayPath)
			_ = os.Symlink(originalLink, disk.Link)
			c.rollbackOverlays(overlays)
			return nil, fmt.Errorf("failed to create overlay symlink %s -> %s: %w", disk.Link, overlayPath, err)
		}

		overlays = append(overlays, &Overlay{
			Path:         overlayPath,
			BackingPath:  disk.Path,
			Disk:         disk,
			OriginalLink: originalLink,
		})
		fmt.Printf("Created overlay %s backed by %s\n", filepath.Base(overlayPath), disk.Path)
	}
	return overlays, nil
}

// rollbackOverlays removes overlay files and restores symlinks for
// partially-created overlays during a failed CreateOverlays call.
func (c *Conversion) rollbackOverlays(overlays []*Overlay) {
	for _, o := range overlays {
		_ = os.Remove(o.Path)
	}
	c.restoreLinks(overlays)
}

// CommitOverlays merges each overlay back into its backing disk, then removes the overlay file.
// Note: commits are not transactional across disks — if commit fails for a later disk,
// earlier disks will already have their changes persisted - qemu-img commit is irreversible
// and each disk remains individually consistent.
func (c *Conversion) CommitOverlays(overlays []*Overlay) error {
	var committed []string
	for _, o := range overlays {
		// Merge dirty clusters from overlay into the base disk
		cmd := c.CommandBuilder.New("qemu-img").
			AddPositional("commit").
			AddPositional(o.Path).
			Build()
		cmd.SetStdout(os.Stdout)
		cmd.SetStderr(os.Stderr)
		if err := cmd.Run(); err != nil {
			c.rollbackOverlays(overlays)
			if len(committed) > 0 {
				return fmt.Errorf("partial commit: disks %v already committed, failed on %s — VM may be in inconsistent state: %w",
					committed, filepath.Base(o.Path), err)
			}
			return fmt.Errorf("failed to commit overlay %s: %w", o.Path, err)
		}
		committed = append(committed, filepath.Base(o.Path))
		// Remove the now-redundant overlay file
		if err := os.Remove(o.Path); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "WARNING: committed overlay %s but failed to remove file: %v\n", filepath.Base(o.Path), err)
		} else {
			fmt.Printf("Committed and removed overlay %s\n", filepath.Base(o.Path))
		}
	}
	// Restore symlinks to point back at base disks
	c.restoreLinks(overlays)
	return nil
}

// DiscardOverlays removes overlay files without committing; base disks are untouched.
func (c *Conversion) DiscardOverlays(overlays []*Overlay) {
	for _, o := range overlays {
		if err := os.Remove(o.Path); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "WARNING: failed to remove overlay %s: %v\n", o.Path, err)
		} else {
			fmt.Printf("Discarded overlay %s (base disk unchanged)\n", filepath.Base(o.Path))
		}
	}
	c.restoreLinks(overlays)
}

func (c *Conversion) restoreLinks(overlays []*Overlay) {
	for _, o := range overlays {
		if err := os.Remove(o.Disk.Link); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "WARNING: failed to remove symlink %s during restore: %v\n", o.Disk.Link, err)
			continue
		}
		if err := os.Symlink(o.OriginalLink, o.Disk.Link); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: failed to restore symlink %s -> %s: %v\n", o.Disk.Link, o.OriginalLink, err)
		}
	}
}

func (c *Conversion) RunInPlaceWithOverlay(runConversion func() error) error {
	// Create overlay per disk and redirect symlinks to overlays
	overlays, err := c.CreateOverlays()
	if err != nil {
		return fmt.Errorf("overlay setup failed: %w", err)
	}

	completed := false
	defer func() {
		if !completed {
			fmt.Println("Unexpected exit — discarding overlays as safety cleanup")
			c.DiscardOverlays(overlays)
		}
	}()

	// Run the actual conversion (writes go to overlays, not base disks)
	convErr := runConversion()

	if convErr != nil {
		fmt.Println("Conversion failed — discarding overlays, base disks unchanged")
		c.DiscardOverlays(overlays)
		completed = true
		return convErr
	}

	fmt.Println("Conversion succeeded — committing overlays to base disks")
	commitErr := c.CommitOverlays(overlays)
	completed = true
	return commitErr
}
