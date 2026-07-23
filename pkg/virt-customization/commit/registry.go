package commit

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

// Registry runs virt-win-reg --merge to apply Windows Registry changes
// to an offline guest disk. Each RegAction's Content is written to a
// temporary .reg file and merged in order.
func Registry(cmdBuilder utils.CommandBuilder, disks []string, keys []string, actions []api.RegAction) error {
	if len(actions) == 0 {
		return nil
	}
	if len(disks) == 0 {
		return fmt.Errorf("no disks provided for virt-win-reg")
	}

	tmpDir, err := os.MkdirTemp("", "virt-win-reg-*")
	if err != nil {
		return fmt.Errorf("creating temp dir for registry files: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	var regFiles []string
	for i, action := range actions {
		name := filepath.Join(tmpDir, fmt.Sprintf("%04d.reg", i+1))
		if err := os.WriteFile(name, action.Content, 0600); err != nil {
			return fmt.Errorf("writing temp registry file %s: %w", name, err)
		}
		regFiles = append(regFiles, name)
	}

	cmd := cmdBuilder.New("virt-win-reg")
	cmd.AddFlag("--merge")
	for _, disk := range disks {
		cmd.AddArg("-a", disk)
	}
	for _, key := range keys {
		cmd.AddArg("--key", key)
	}
	for _, rf := range regFiles {
		cmd.AddPositional(rf)
	}

	builtCmd := cmd.Build()
	builtCmd.SetStdout(os.Stdout)
	builtCmd.SetStderr(os.Stderr)
	if err := builtCmd.Run(); err != nil {
		return fmt.Errorf("virt-win-reg merge: %w", err)
	}
	return nil
}
