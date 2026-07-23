// Package guesthandle provides the production GuestHandle backed by
// the libguestfs Go (CGO) bindings. Build-tagged so that unit tests
// not needing a real appliance can compile without libguestfs-devel.
//
//go:build ignore

package guesthandle

import (
	"fmt"
	"sort"

	"libguestfs.org/guestfs"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// Handle wraps a live libguestfs session and implements api.GuestHandle.
type Handle struct {
	g *guestfs.Guestfs
}

// Open creates a new libguestfs session, adds the given disks (raw format),
// launches the appliance, inspects for an OS root, and mounts all
// discovered filesystems. The returned Handle is ready for probe and
// file operations. Callers must call Shutdown + Close when done.
func Open(disks []string, keys []string, rootDisk string) (api.GuestHandle, error) {
	if len(disks) == 0 {
		return nil, fmt.Errorf("guesthandle.Open: no disks provided")
	}

	g, errno := guestfs.Create()
	if errno != nil {
		return nil, fmt.Errorf("guestfs.Create: %w", errno)
	}

	for _, disk := range disks {
		optargs := guestfs.OptargsAdd_drive{
			Format_is_set:   true,
			Format:          "raw",
			Readonly_is_set: true,
			Readonly:        false,
		}
		if err := g.Add_drive(disk, &optargs); err != nil {
			g.Close()
			return nil, fmt.Errorf("add_drive %s: %w", disk, err)
		}
	}

	if err := g.Launch(); err != nil {
		g.Close()
		return nil, fmt.Errorf("launch: %w", err)
	}

	roots, err := g.Inspect_os()
	if err != nil {
		g.Close()
		return nil, fmt.Errorf("inspect_os: %w", err)
	}

	root := pickRoot(roots, rootDisk)
	if root == "" {
		g.Close()
		return nil, fmt.Errorf("no OS root found (inspected %d candidates)", len(roots))
	}

	mountpoints, err := g.Inspect_get_mountpoints(root)
	if err != nil {
		g.Close()
		return nil, fmt.Errorf("inspect_get_mountpoints: %w", err)
	}

	// Mount in path-length order so parents appear before children.
	mps := sortedMountpoints(mountpoints)
	for _, mp := range mps {
		if err := g.Mount(mp.device, mp.mountpoint); err != nil {
			fmt.Printf("warning: mount %s on %s: %v\n", mp.device, mp.mountpoint, err)
		}
	}

	// LUKS keys are handled at the appliance level; placeholder for future support.
	_ = keys

	return &Handle{g: g}, nil
}

func (h *Handle) IsDir(path string) (bool, error)            { return h.g.Is_dir(path, nil) }
func (h *Handle) IsFile(path string) (bool, error)            { return h.g.Is_file(path, nil) }
func (h *Handle) Cat(path string) (string, error)             { return h.g.Cat(path) }
func (h *Handle) Ls(dir string) ([]string, error)             { return h.g.Ls(dir) }
func (h *Handle) MkdirP(path string) error                    { return h.g.Mkdir_p(path) }
func (h *Handle) Chmod(mode int, path string) error            { return h.g.Chmod(mode, path) }
func (h *Handle) Write(path string, content []byte) error     { return h.g.Write(path, content) }
func (h *Handle) Upload(local, guest string) error             { return h.g.Upload(local, guest) }
func (h *Handle) ReadFile(path string) ([]byte, error)         { return h.g.Read_file(path) }
func (h *Handle) GlobExpand(pattern string) ([]string, error) { return h.g.Glob_expand(pattern, nil) }
func (h *Handle) Shutdown() error                              { return h.g.Shutdown() }
func (h *Handle) Close()                                       { h.g.Close() }

var _ api.GuestHandle = (*Handle)(nil)

func pickRoot(roots []string, rootDisk string) string {
	if len(roots) == 0 {
		return ""
	}
	if rootDisk != "" {
		for _, r := range roots {
			if r == rootDisk {
				return r
			}
		}
	}
	return roots[0]
}

type mountEntry struct {
	mountpoint string
	device     string
}

func sortedMountpoints(mp map[string]string) []mountEntry {
	entries := make([]mountEntry, 0, len(mp))
	for mountpoint, device := range mp {
		entries = append(entries, mountEntry{mountpoint: mountpoint, device: device})
	}
	sort.Slice(entries, func(i, j int) bool {
		return len(entries[i].mountpoint) < len(entries[j].mountpoint)
	})
	return entries
}
