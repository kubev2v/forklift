package commit

import (
	"fmt"
	"os"

	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

// Scripts runs virt-customize with --firstboot and --run actions.
// Keys are passed for LUKS encrypted volumes.
func Scripts(cmdBuilder utils.CommandBuilder, disks []string, keys []string, actions []api.ExecAction) error {
	if len(actions) == 0 {
		return nil
	}
	if len(disks) == 0 {
		return fmt.Errorf("no disks provided for virt-customize")
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
	for i, action := range actions {
		switch action.Type {
		case api.ActionFirstboot:
			cmd.AddArg("--firstboot", action.Value)
		case api.ActionRun:
			cmd.AddArg("--run", action.Value)
		default:
			return fmt.Errorf("unsupported exec action type %q at index %d (value=%q)", action.Type, i, action.Value)
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
