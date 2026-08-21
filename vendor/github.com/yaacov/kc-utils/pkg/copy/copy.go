// Package copy provides standalone vSphere disk copy via govmomi NFC export.
// It downloads VMDK disks from vCenter/ESXi over HTTPS (no VDDK required)
// and writes raw data to PVC targets (block devices or filesystem images)
// or to `{target_dir}/diskN.img`.
package copy

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"

	"github.com/yaacov/kc-utils/pkg/common/types"
	v2vtls "github.com/yaacov/kc-utils/pkg/v2v/tls"
)

const DefaultCopyConcurrency = 4

// DefaultWorkdir is the default working directory for copy progress files.
const DefaultWorkdir = "/var/tmp/v2v"

// CopyInput is the standalone input to Run.
type CopyInput struct {
	Host            string   `json:"host"`
	Datacenter      string   `json:"datacenter,omitempty"`
	Insecure        bool     `json:"insecure"`
	CaCert          string   `json:"ca_cert,omitempty"`
	VMName          string   `json:"vm_name"`
	Fingerprint     string   `json:"fingerprint"`
	SourceDisks     []string `json:"source_disks"` // VMDK paths to copy; empty = all NFC lease disks (list order → target index)
	TargetDir       string   `json:"target_dir,omitempty"`
	Workdir         string   `json:"workdir"`
	OutputPath      string   `json:"output_path,omitempty"`
	CopyConcurrency int      `json:"copy_concurrency,omitempty"`
}

// Progress tracks per-disk copy status.
type Progress struct {
	Disks []DiskProgress `json:"disks"`
}

// DiskProgress is one disk copy result.
type DiskProgress struct {
	Index      int    `json:"index"`
	SourceFile string `json:"source_file"`
	Target     string `json:"target"`
	Status     string `json:"status"`
}

// ClampConcurrency returns a sane worker count for parallel disk copy.
func ClampConcurrency(n, disks int) int {
	if n <= 0 {
		n = DefaultCopyConcurrency
	}
	if disks < 1 {
		return 1
	}
	if n > disks {
		return disks
	}
	return n
}

// Run copies vSphere disks into empty PVC targets or `{target_dir}/diskN.img`.
func Run(input *CopyInput) error {
	if input.Host == "" {
		return fmt.Errorf("host is required")
	}
	if input.VMName == "" {
		return fmt.Errorf("vm_name is required")
	}
	if input.Fingerprint == "" {
		return fmt.Errorf("fingerprint is required")
	}

	var targets []Target
	if input.TargetDir == "" {
		allTargets, err := DiscoverTargets()
		if err != nil {
			return err
		}
		if err := logDiscoveredTargets(allTargets); err != nil {
			return err
		}

		targets, err = EmptyTargets()
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			return fmt.Errorf("no empty PVC targets found")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Info("NFC disk copy starting",
		"source_disks", len(input.SourceDisks),
		"all_disks", len(input.SourceDisks) == 0,
		"target_dir", input.TargetDir,
		"empty_targets", len(targets),
		"vm", input.VMName,
	)

	outputPath := input.OutputPath
	if outputPath == "" {
		outputPath = input.Workdir + "/copy-progress.json"
	}

	policy, err := v2vtls.CopyTLS(input.Insecure, input.CaCert)
	if err != nil {
		return err
	}

	lease, err := ExportVM(ctx, input.Host, input.Datacenter, policy, input.Fingerprint, input.VMName)
	if err != nil {
		return fmt.Errorf("NFC export: %w", err)
	}

	selected, err := FilterDiskURLs(lease.DiskURLs, input.SourceDisks)
	if err != nil {
		_ = lease.Abort(ctx)
		return fmt.Errorf("filter NFC disks: %w", err)
	}
	if input.TargetDir != "" {
		if len(selected) == 0 {
			_ = lease.Abort(ctx)
			return fmt.Errorf("no disks in NFC lease")
		}
		if err := os.MkdirAll(input.TargetDir, 0o755); err != nil {
			_ = lease.Abort(ctx)
			return fmt.Errorf("create target_dir %s: %w", input.TargetDir, err)
		}
		targets = TargetsFromDir(input.TargetDir, len(selected))
		slog.Info("using copy target dir", "dir", input.TargetDir, "disks", len(targets))
		for _, t := range targets {
			slog.Info("copy target",
				"index", t.Index,
				"path", t.Path,
				"kind", "file",
			)
		}
	} else if len(selected) != len(targets) {
		_ = lease.Abort(ctx)
		return fmt.Errorf("disk count mismatch: %d selected source disk(s) vs %d empty target(s)", len(selected), len(targets))
	}

	total := len(targets)
	concurrency := ClampConcurrency(input.CopyConcurrency, total)
	slog.Info("NFC disks selected",
		"selected", total,
		"lease", len(lease.DiskURLs),
		"concurrency", concurrency,
	)

	progress := Progress{}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	var errOnce sync.Once

	for i, target := range targets {
		i, target := i, target
		diskURL := selected[i]
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			slog.Info("copying disk via NFC",
				"disk", fmt.Sprintf("%d/%d", i+1, total),
				"index", target.Index,
				"source", diskURL.DiskPath,
				"target", target.Path,
				"block", target.IsBlockDev,
			)
			if err := CopyDisk(ctx, lease, diskURL, target, func(pct int) {
				slog.Info("disk copy progress",
					"index", target.Index,
					"source", diskURL.DiskPath,
					"target", target.Path,
					"percent", pct,
				)
			}); err != nil {
				errOnce.Do(func() {
					firstErr = err
					cancel()
				})
				return
			}

			// Reclaim per-disk buffers (compBuf, grainBuf, decompressor)
			// before the semaphore slot is released to the next disk.
			runtime.GC()

			mu.Lock()
			defer mu.Unlock()
			progress.Disks = append(progress.Disks, DiskProgress{
				Index:      target.Index,
				SourceFile: diskURL.DiskPath,
				Target:     target.Path,
				Status:     "complete",
			})
			if err := types.WriteJSON(outputPath, progress); err != nil {
				slog.Warn("failed to write copy progress", "error", err)
			}
			slog.Info("disk copy recorded",
				"completed", fmt.Sprintf("%d/%d", len(progress.Disks), total),
				"index", target.Index,
			)
		}()
	}

	wg.Wait()

	if firstErr != nil {
		_ = lease.Abort(ctx)
		return firstErr
	}

	if err := lease.Complete(ctx); err != nil {
		slog.Warn("NFC lease complete failed", "error", err)
	}

	slog.Info("NFC disk copy complete", "disks", total)
	return nil
}

func logDiscoveredTargets(targets []Target) error {
	slog.Info("discovered PVC targets", "count", len(targets))
	for _, t := range targets {
		empty, err := isTargetEmpty(t)
		if err != nil {
			return err
		}
		kind := "filesystem"
		if t.IsBlockDev {
			kind = "block"
		}
		slog.Info("copy target",
			"index", t.Index,
			"path", t.Path,
			"kind", kind,
			"empty", empty,
		)
	}
	return nil
}
