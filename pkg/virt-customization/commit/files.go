package commit

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

var validOctalMode = regexp.MustCompile(`^[0-7]{3,4}$`)

// Files performs all file operations (uploads and writes) on the guest
// disk via the GuestHandle.
//
// ActionUpload is translated to g.Upload(localpath, guestpath).
// ActionWrite is translated to g.Write(guestpath, content) directly —
// no temp file staging is needed.
// Parent directories are created with g.MkdirP and optional permissions
// are applied with g.Chmod.
func Files(g api.GuestHandle, actions []api.FileAction) error {
	if len(actions) == 0 {
		return nil
	}

	for _, action := range actions {
		if action.Permissions != "" && !validOctalMode.MatchString(action.Permissions) {
			return fmt.Errorf("invalid permissions %q for %s: must be an octal mode (e.g. 0644)", action.Permissions, action.GuestPath)
		}
		if action.Type != api.ActionUpload && action.Type != api.ActionWrite {
			return fmt.Errorf("unsupported file action type: %v", action.Type)
		}
	}

	// Ensure parent directories exist before writing files.
	// Uses path (not filepath) because guest paths use forward slashes.
	dirs := map[string]bool{}
	for _, action := range actions {
		dir := path.Dir(action.GuestPath)
		if dir != "" && dir != "." && dir != "/" {
			dirs[dir] = true
		}
	}
	if len(dirs) > 0 {
		sorted := make([]string, 0, len(dirs))
		for d := range dirs {
			sorted = append(sorted, d)
		}
		sort.Strings(sorted)
		for _, d := range sorted {
			if err := g.MkdirP(d); err != nil {
				return fmt.Errorf("mkdir_p %s: %w", d, err)
			}
		}
	}

	for _, action := range actions {
		switch action.Type {
		case api.ActionUpload:
			if err := g.Upload(action.LocalPath, action.GuestPath); err != nil {
				return fmt.Errorf("upload %s -> %s: %w", action.LocalPath, action.GuestPath, err)
			}
		case api.ActionWrite:
			if err := g.Write(action.GuestPath, action.Content); err != nil {
				return fmt.Errorf("write %s: %w", action.GuestPath, err)
			}
		}

		if action.Permissions != "" {
			mode, _ := strconv.ParseInt(action.Permissions, 8, 32)
			if err := g.Chmod(int(mode), action.GuestPath); err != nil {
				return fmt.Errorf("chmod %s on %s: %w", action.Permissions, action.GuestPath, err)
			}
		}
	}

	return nil
}
