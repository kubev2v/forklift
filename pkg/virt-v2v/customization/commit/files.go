package commit

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

var validOctalMode = regexp.MustCompile(`^[0-7]{3,4}$`)

// Files runs a single guestfish --rw session to perform all file
// operations (uploads and writes) on the guest disk.
func Files(cmdBuilder utils.CommandBuilder, disks []string, keys []string, rootDisk string, actions []api.FileAction) error {
	if len(actions) == 0 {
		return nil
	}

	script, err := buildScript(actions)
	if err != nil {
		return fmt.Errorf("building guestfish script: %w", err)
	}

	cmd := cmdBuilder.New("guestfish")
	cmd.AddFlag("--rw")
	for _, disk := range disks {
		cmd.AddArg("-a", disk)
	}
	for _, key := range keys {
		cmd.AddArg("--key", key)
	}
	if rootDisk != "" {
		cmd.AddArg("--root", rootDisk)
	} else {
		cmd.AddArg("--root", "first")
	}
	cmd.AddFlag("-i")

	builtCmd := cmd.Build()
	builtCmd.SetStdin(strings.NewReader(script))
	builtCmd.SetStdout(os.Stdout)
	builtCmd.SetStderr(os.Stderr)

	if err := builtCmd.Run(); err != nil {
		return fmt.Errorf("guestfish apply: %w", err)
	}
	return nil
}

func buildScript(actions []api.FileAction) (string, error) {
	var buf bytes.Buffer

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
			fmt.Fprintf(&buf, "mkdir-p %s\n", strconv.Quote(d))
		}
	}

	for _, action := range actions {
		switch action.Type {
		case api.ActionUpload:
			fmt.Fprintf(&buf, "upload %s %s\n", strconv.Quote(action.LocalPath), strconv.Quote(action.GuestPath))
		case api.ActionWrite:
			// strconv.Quote escapes are compatible with guestfish double-quoted strings,
			// except for non-ASCII runes which Go renders as \uNNNN (unsupported by guestfish).
			// In practice content here is ASCII config/script text; use ActionUpload for binary data.
			fmt.Fprintf(&buf, "write %s %s\n", strconv.Quote(action.GuestPath), strconv.Quote(string(action.Content)))
		default:
			return "", fmt.Errorf("unsupported file action type: %v", action.Type)
		}
		if action.Permissions != "" {
			if !validOctalMode.MatchString(action.Permissions) {
				return "", fmt.Errorf("invalid permissions %q for %s: must be an octal mode (e.g. 0644)", action.Permissions, action.GuestPath)
			}
			fmt.Fprintf(&buf, "chmod %s %s\n", action.Permissions, strconv.Quote(action.GuestPath))
		}
	}
	return buf.String(), nil
}
