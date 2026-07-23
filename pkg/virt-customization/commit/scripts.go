package commit

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

// Scripts runs virt-customize with --firstboot and --run actions.
// Keys are passed for LUKS encrypted volumes.
//
// Actions that carry inline Content are written to temp files automatically;
// actions with ScriptPath reference pre-existing host files directly.
func Scripts(cmdBuilder utils.CommandBuilder, disks []string, keys []string, actions []api.ExecAction) error {
	if len(actions) == 0 {
		return nil
	}
	if len(disks) == 0 {
		return fmt.Errorf("no disks provided for virt-customize")
	}

	// Resolve inline Content to temp files so virt-customize can read them.
	resolved, tmpDir, err := resolveScriptPaths(actions)
	if err != nil {
		return err
	}
	if tmpDir != "" {
		defer func() { _ = os.RemoveAll(tmpDir) }()
	}

	cmd := cmdBuilder.New("virt-customize")
	cmd.AddFlag("--verbose")
	cmd.AddArg("--format", "raw")
	for _, disk := range disks {
		cmd.AddArg("-a", disk)
	}
	for _, key := range keys {
		cmd.AddArg("--key", key)
	}
	for i, action := range resolved {
		switch action.Type {
		case api.ActionFirstboot:
			cmd.AddArg("--firstboot", action.ScriptPath)
		case api.ActionRun:
			cmd.AddArg("--run", action.ScriptPath)
		default:
			return fmt.Errorf("unsupported exec action type %q at index %d (value=%q)", action.Type, i, action.ScriptPath)
		}
	}

	builtCmd := cmd.Build()
	builtCmd.SetStdout(os.Stdout)
	builtCmd.SetStderr(os.Stderr)
	if err := builtCmd.Run(); err != nil {
		return fmt.Errorf("virt-customize exec: %w", err)
	}
	return nil
}

// resolveScriptPaths returns a copy of actions where any inline Content has
// been written to a temp file and ScriptPath set accordingly. A single temp
// directory is created only if needed and returned so the caller can clean up.
func resolveScriptPaths(actions []api.ExecAction) ([]api.ExecAction, string, error) {
	var tmpDir string
	resolved := make([]api.ExecAction, len(actions))
	copy(resolved, actions)

	for i := range resolved {
		if len(resolved[i].Content) == 0 {
			continue
		}
		if tmpDir == "" {
			var err error
			tmpDir, err = os.MkdirTemp("", "virt-customize-scripts-*")
			if err != nil {
				return nil, "", fmt.Errorf("creating temp dir for exec scripts: %w", err)
			}
		}
		name := filepath.Join(tmpDir, fmt.Sprintf("%04d-script", i+1))
		if err := os.WriteFile(name, resolved[i].Content, 0700); err != nil {
			return nil, tmpDir, fmt.Errorf("writing temp script %s: %w", name, err)
		}
		resolved[i].ScriptPath = name
	}
	return resolved, tmpDir, nil
}
