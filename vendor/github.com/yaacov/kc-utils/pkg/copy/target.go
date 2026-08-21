package copy

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/yaacov/kc-utils/pkg/common/types"
)

const emptyThreshold = 1 << 20 // 1 MiB

var (
	blockGlob = "/dev/block[0-9]*"
	fsGlob    = "/mnt/disks/disk[0-9]*"
)

// SetTargetGlobs overrides target discovery globs for tests. Returns a restore func.
func SetTargetGlobs(block, fs string) func() {
	oldBlock, oldFS := blockGlob, fsGlob
	blockGlob, fsGlob = block, fs
	return func() {
		blockGlob, fsGlob = oldBlock, oldFS
	}
}

// Target describes a PVC write destination.
type Target struct {
	Path       string
	IsBlockDev bool
	Index      int
}

var diskNumRE = regexp.MustCompile(`\d+`)

// DiscoverTargets finds conversion-pod PVC paths (block or filesystem).
func DiscoverTargets() ([]Target, error) {
	block, err := filepath.Glob(blockGlob)
	if err != nil {
		return nil, err
	}
	fsDirs, err := filepath.Glob(fsGlob)
	if err != nil {
		return nil, err
	}

	var targets []Target
	for _, p := range block {
		targets = append(targets, Target{
			Path:       p,
			IsBlockDev: true,
			Index:      diskIndex(p),
		})
	}
	for _, dir := range fsDirs {
		targets = append(targets, Target{
			Path:       filepath.Join(dir, "disk.img"),
			IsBlockDev: false,
			Index:      diskIndex(dir),
		})
	}
	if len(targets) == 0 {
		return nil, nil
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Index < targets[j].Index
	})
	return targets, nil
}

// TargetsFromDir builds file targets {dir}/disk0.img … disk{n-1}.img.
func TargetsFromDir(dir string, n int) []Target {
	if n < 1 {
		return nil
	}
	targets := make([]Target, n)
	for i := 0; i < n; i++ {
		targets[i] = Target{
			Path:       filepath.Join(dir, types.ImageFileName(i)),
			IsBlockDev: false,
			Index:      i,
		}
	}
	return targets
}

// HasEmptyTargets reports whether any conversion-pod mount point exists with empty content.
func HasEmptyTargets() (bool, error) {
	targets, err := DiscoverTargets()
	if err != nil {
		return false, err
	}
	for _, t := range targets {
		empty, err := isTargetEmpty(t)
		if err != nil {
			return false, err
		}
		if empty {
			return true, nil
		}
	}
	return false, nil
}

// EmptyTargets returns targets that still need data copied.
func EmptyTargets() ([]Target, error) {
	targets, err := DiscoverTargets()
	if err != nil {
		return nil, err
	}
	var empty []Target
	for _, t := range targets {
		isEmpty, err := IsTargetEmpty(t)
		if err != nil {
			return nil, err
		}
		if isEmpty {
			empty = append(empty, t)
		}
	}
	return empty, nil
}

// IsTargetEmpty reports whether a PVC target still needs data copied.
func IsTargetEmpty(t Target) (bool, error) {
	return isTargetEmpty(t)
}

func isTargetEmpty(t Target) (bool, error) {
	info, err := os.Stat(t.Path)
	if err != nil {
		if os.IsNotExist(err) {
			if t.IsBlockDev {
				return false, nil
			}
			return true, nil
		}
		return false, err
	}
	if t.IsBlockDev {
		return isBlockEmpty(t.Path, info.Size())
	}
	return info.Size() < emptyThreshold, nil
}

const emptyCheckChunk = 64 << 10 // 64 KiB per read

func isBlockEmpty(path string, size int64) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Linux reports st_size=0 for block devices; treat that as unknown size and
	// always probe the first emptyThreshold bytes instead of assuming empty.
	remaining := emptyThreshold
	if size > 0 && size < int64(remaining) {
		remaining = int(size)
	}
	var buf [emptyCheckChunk]byte
	for remaining > 0 {
		toRead := remaining
		if toRead > emptyCheckChunk {
			toRead = emptyCheckChunk
		}
		n, err := f.Read(buf[:toRead])
		if n > 0 {
			for _, b := range buf[:n] {
				if b != 0 {
					return false, nil
				}
			}
			remaining -= n
		}
		if err == io.EOF {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if n == 0 {
			return true, nil
		}
	}
	return true, nil
}

func diskIndex(path string) int {
	n, _ := strconv.Atoi(diskNumRE.FindString(path))
	return n
}
