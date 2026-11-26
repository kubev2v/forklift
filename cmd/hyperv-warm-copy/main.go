// Command hyperv-warm-copy performs one warm-migration copy pass for a
// Hyper-V source VM, writing each source disk onto a raw block device.
//
//	full   convert the base VHDX with qemu-img (iteration 0)
//	delta  copy only the blocks a frozen AVHDX holds (iterations 1..N,
//	       and the final pass after cutover)
//
// Work is described by the DISK_ARGS environment variable, one disk per
// line, fields separated by "|":
//
//	mode|src|parent|dev
//
// for example:
//
//	full|/hyperv/vm.vhdx||/dev/block0
//	delta|/hyperv/vm_A1B2.avhdx|/hyperv/vm.vhdx|/dev/block0
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/lib/hyperv/vhdx"
	"k8s.io/klog/v2"
)

const (
	modeFull  = "full"
	modeDelta = "delta"
)

// diskJob is one line of DISK_ARGS.
type diskJob struct {
	mode   string
	src    string
	parent string
	dev    string
}

func main() {
	klog.InitFlags(nil)

	var (
		diskArgs = flag.String("disk-args", os.Getenv("DISK_ARGS"),
			"newline-separated mode|src|parent|dev records; defaults to $DISK_ARGS")
		stageDir = flag.String("stage-dir", "/tmp",
			"directory used to stage an AVHDX off the read-only SMB mount before parsing")
		stage = flag.Bool("stage", true,
			"copy the AVHDX to -stage-dir before parsing; the SMB mount is read-only and "+
				"can present the file with restrictive permissions")
		bufSize = flag.Int("buffer-size", vhdx.DefaultCopyBuffer,
			"transfer size in bytes for delta range copies")
	)
	flag.Parse()

	jobs, err := parseDiskArgs(*diskArgs)
	if err != nil {
		klog.Fatal(err)
	}
	klog.Infof("warm copy starting: %d disk(s)", len(jobs))

	for i, job := range jobs {
		start := time.Now()
		klog.Infof("[%d/%d] %s copy: %s -> %s", i+1, len(jobs), job.mode, job.src, job.dev)
		if err := run(job, *stage, *stageDir, *bufSize); err != nil {
			klog.Fatalf("[%d/%d] %s copy %s -> %s failed: %v",
				i+1, len(jobs), job.mode, job.src, job.dev, err)
		}
		klog.Infof("[%d/%d] %s copy done in %s", i+1, len(jobs), job.mode,
			time.Since(start).Round(time.Millisecond))
	}
	klog.Info("all copies completed")
}

func run(job diskJob, stage bool, stageDir string, bufSize int) error {
	switch job.mode {
	case modeFull:
		return fullCopy(job)
	case modeDelta:
		return deltaCopy(job, stage, stageDir, bufSize)
	default:
		return fmt.Errorf("unknown mode %q", job.mode)
	}
}

// parseDiskArgs splits the DISK_ARGS payload into jobs, rejecting records
// that would otherwise fail deep inside a copy.
func parseDiskArgs(raw string) ([]diskJob, error) {
	var jobs []diskJob
	for n, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) != 4 {
			return nil, fmt.Errorf("DISK_ARGS line %d: want 4 |-separated fields, got %d: %q",
				n+1, len(fields), line)
		}
		job := diskJob{
			mode:   strings.TrimSpace(fields[0]),
			src:    strings.TrimSpace(fields[1]),
			parent: strings.TrimSpace(fields[2]),
			dev:    strings.TrimSpace(fields[3]),
		}
		if job.mode != modeFull && job.mode != modeDelta {
			return nil, fmt.Errorf("DISK_ARGS line %d: unknown mode %q", n+1, job.mode)
		}
		if job.src == "" || job.dev == "" {
			return nil, fmt.Errorf("DISK_ARGS line %d: src and dev are required: %q", n+1, line)
		}
		jobs = append(jobs, job)
	}
	if len(jobs) == 0 {
		return nil, errors.New("DISK_ARGS is empty; nothing to copy")
	}
	return jobs, nil
}

// fullCopy: qemu-img convert of a standalone VHDX/VHD. Rejects .avhdx/.avhd
// (qemu-img cannot follow Hyper-V differencing parents).
func fullCopy(job diskJob) error {
	ext := strings.ToLower(filepath.Ext(job.src))
	if ext == ".avhdx" || ext == ".avhd" {
		return fmt.Errorf("full copy refuses differencing disk %s; qemu-img cannot merge Hyper-V VHDX parent chains",
			job.src)
	}
	// qemu-img names the legacy fixed/dynamic VHD format "vpc".
	format := "vhdx"
	if ext == ".vhd" {
		format = "vpc"
	}
	// Absolute path: the virt-v2v image installs qemu-img here, and using PATH
	// would trip Sonar S4036 (untrusted PATH directories).
	cmd := exec.Command("/usr/bin/qemu-img", "convert",
		"-p", // progress, surfaced in the pod log
		"-f", format,
		"-O", "raw",
		"-t", "writeback",
		"-W", // out-of-order writes
		job.src, job.dev)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	klog.Infof("exec: %s", strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("qemu-img convert: %w", err)
	}
	return syncDevice(job.dev)
}

// deltaCopy writes only the blocks the frozen AVHDX holds, leaving every
// other extent of the target as the preceding passes left it.
func deltaCopy(job diskJob, stage bool, stageDir string, bufSize int) error {
	src := job.src
	if stage {
		staged, cleanup, err := stageLocally(job.src, stageDir)
		if err != nil {
			return err
		}
		defer cleanup()
		src = staged
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open AVHDX: %w", err)
	}
	defer func() { _ = in.Close() }()

	ranges, geo, err := vhdx.ChangedRanges(in)
	if err != nil {
		return err
	}
	klog.Infof("AVHDX geometry: block=%dMiB sector=%d virtual=%dMiB chunkRatio=%d blocks=%d",
		geo.BlockSize>>20, geo.SectorSize, geo.VirtualSize>>20, geo.ChunkRatio, geo.TotalBlocks)

	if len(ranges) == 0 {
		klog.Info("no changed blocks in delta; target left untouched")
		return nil
	}
	klog.Infof("%d range(s), %.1f MiB to copy",
		len(ranges), float64(vhdx.TotalBytes(ranges))/(1<<20))

	out, err := os.OpenFile(job.dev, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open target device: %w", err)
	}
	defer func() { _ = out.Close() }()

	written, err := vhdx.Copy(out, in, ranges, bufSize)
	if err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return fmt.Errorf("sync target device: %w", err)
	}
	klog.Infof("delta copy wrote %.1f MiB", float64(written)/(1<<20))
	return nil
}

// stageLocally copies the AVHDX from the SMB mount into a local temp file
// so BAT walks and range reads hit local disk instead of slow SMB random I/O.
// Returns the temp path and a cleanup that removes it. Use -stage=false to skip.
func stageLocally(src, dir string) (path string, cleanup func(), err error) {
	in, err := os.Open(src)
	if err != nil {
		return "", nil, fmt.Errorf("open source AVHDX: %w", err)
	}
	defer func() { _ = in.Close() }()

	tmp, err := os.CreateTemp(dir, "delta-*.avhdx")
	if err != nil {
		return "", nil, fmt.Errorf("create staging file: %w", err)
	}
	path = tmp.Name()
	cleanup = func() {
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			klog.Warningf("remove staging file %s: %v", path, rmErr)
		}
	}

	klog.Infof("staging %s -> %s", src, path)
	n, copyErr := io.Copy(tmp, in)
	closeErr := tmp.Close()
	if copyErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("stage AVHDX: %w", copyErr)
	}
	if closeErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("close staging file: %w", closeErr)
	}
	klog.Infof("staged %.1f MiB", float64(n)/(1<<20))
	return path, cleanup, nil
}

// syncDevice flushes the target device so the pod does not report success
// before the data is durable.
func syncDevice(dev string) error {
	f, err := os.OpenFile(dev, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s to sync: %w", dev, err)
	}
	defer func() { _ = f.Close() }()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", dev, err)
	}
	return nil
}
