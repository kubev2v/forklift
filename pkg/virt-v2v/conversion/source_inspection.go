package conversion

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

// RunSourceV2vInspection inspects a vSphere guest before conversion.
// virt-v2v-open runs virt-inspector inside --run, expanding @@ to each disk's
// NBD URI.
func (c *Conversion) RunSourceV2vInspection() error {
	runArgs := []string{"virt-inspector", "-v", "-x", "--format=raw"}
	if c.NbdeClevis {
		runArgs = append(runArgs, "--key", "all:clevis")
	} else if c.Luksdir != "" {
		if _, err := c.fileSystem.Stat(c.Luksdir); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("error accessing the LUKS directory: %w", err)
			}
		} else {
			files, err := utils.GetFilesInPath(c.fileSystem, c.Luksdir)
			if err != nil {
				return fmt.Errorf("error reading files in LUKS directory: %w", err)
			}
			for _, file := range files {
				runArgs = append(runArgs, "--key", fmt.Sprintf("all:file:%s", file))
			}
		}
	}
	runArgs = append(runArgs, c.InspectorExtraArgs...)
	runArgs = append(runArgs, "@@", ">", c.InspectionOutputFile)

	cmd := c.CommandBuilder.New("virt-v2v-open").
		AddFlag("-v").
		AddFlag("-x").
		AddArg("--run", strings.Join(runArgs, " ")).
		AddArg("-i", "libvirt").
		AddArg("-ic", c.LibvirtUrl).
		AddArg("-ip", c.SecretKey)
	if c.addVsphereInputTransport(cmd) == vsphereTransportVddk {
		if _, err := os.Stat(c.VddkConfFile); !errors.Is(err, os.ErrNotExist) {
			cmd.AddArg("-io", fmt.Sprintf("vddk-config=%s", c.VddkConfFile))
		}
	}
	cmd.AddPositional("--").AddPositional(c.VmName)

	executor := cmd.Build()
	executor.SetStdout(os.Stdout)
	executor.SetStderr(os.Stderr)
	return executor.Run()
}
