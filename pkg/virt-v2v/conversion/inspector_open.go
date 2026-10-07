package conversion

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

// buildVirtInspectorRunArgs assembles argv for virt-inspector inside virt-v2v-open --run.
// @@ is a literal placeholder expanded by virt-v2v-open into -a <nbd-uri> per disk.
func (c *Conversion) buildVirtInspectorRunArgs() ([]string, error) {
	parts := []string{"virt-inspector", "-v", "-x", "--format=raw"}

	keyParts, err := c.inspectorKeyArgs()
	if err != nil {
		return nil, err
	}
	parts = append(parts, keyParts...)
	parts = append(parts, c.InspectorExtraArgs...)
	parts = append(parts, "@@", ">", c.InspectionOutputFile)

	return parts, nil
}

func (c *Conversion) inspectorKeyArgs() ([]string, error) {
	if c.NbdeClevis {
		return []string{"--key", "all:clevis"}, nil
	}
	if c.Luksdir == "" {
		return nil, nil
	}
	if _, err := c.fileSystem.Stat(c.Luksdir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error accessing the LUKS directory: %v", err)
	}
	files, err := utils.GetFilesInPath(c.fileSystem, c.Luksdir)
	if err != nil {
		return nil, fmt.Errorf("error reading files in LUKS directory: %v", err)
	}
	var parts []string
	for _, file := range files {
		parts = append(parts, "--key", fmt.Sprintf("all:file:%s", file))
	}
	return parts, nil
}

// addVirtV2vOpenVsphereConnectionArgs adds vSphere connection options for virt-v2v-open.
func (c *Conversion) addVirtV2vOpenVsphereConnectionArgs(cmd utils.CommandBuilder) error {
	cmd.AddArg("-i", "libvirt").
		AddArg("-ic", c.LibvirtUrl).
		AddArg("-ip", c.SecretKey)
	if c.addVsphereInputTransport(cmd) == vsphereTransportVddk {
		if _, err := os.Stat(c.VddkConfFile); !errors.Is(err, os.ErrNotExist) {
			cmd.AddArg("-io", fmt.Sprintf("vddk-config=%s", c.VddkConfFile))
		}
	}
	return nil
}

func (c *Conversion) runVirtV2vOpenInspection(
	addConnection func(utils.CommandBuilder) error,
	addGuest func(utils.CommandBuilder),
) error {
	runArgs, err := c.buildVirtInspectorRunArgs()
	if err != nil {
		return err
	}
	cmdBuilder := c.CommandBuilder.New("virt-v2v-open").
		AddFlag("-v").
		AddFlag("-x").
		AddArg("--run", strings.Join(runArgs, " "))
	if err := addConnection(cmdBuilder); err != nil {
		return err
	}
	addGuest(cmdBuilder)
	v2vCmd := cmdBuilder.Build()
	v2vCmd.SetStdout(os.Stdout)
	v2vCmd.SetStderr(os.Stderr)
	return v2vCmd.Run()
}
