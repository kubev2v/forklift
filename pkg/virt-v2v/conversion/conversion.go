package conversion

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customize"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

type Conversion struct {
	*config.AppConfig
	// Disks to be converted
	Disks []*Disk
	// Used for injecting mock to the builder
	CommandBuilder utils.CommandBuilder

	fileSystem utils.FileSystem
}

func NewConversion(env *config.AppConfig) (*Conversion, error) {
	conversion := Conversion{
		AppConfig:      env,
		CommandBuilder: &utils.CommandBuilderImpl{},
		fileSystem:     &utils.FileSystemImpl{},
	}

	disks, err := conversion.getDisk()
	if err != nil {
		return nil, err
	}
	conversion.Disks = disks

	return &conversion, nil
}

func (c *Conversion) getDisk() ([]*Disk, error) {
	var disks []*Disk
	diskPaths, err := filepath.Glob(config.FS)
	if err != nil {
		return nil, err
	}
	disksBlock, err := filepath.Glob(config.BLOCK)
	if err != nil {
		return nil, err
	}
	diskPaths = append(diskPaths, disksBlock...)
	for _, path := range diskPaths {
		disk, err := NewDisk(c.AppConfig, path)
		if err != nil {
			return nil, err
		}
		disks = append(disks, disk)
	}
	return disks, nil
}

// addCommonArgs adds a v2v arguments which is used for both virt-v2v and virt-v2v-in-place
func (c *Conversion) addCommonArgs(cmd utils.CommandBuilder) error {
	// Allow specifying which disk should be the bootable disk
	if c.RootDisk != "" {
		cmd.AddArg("--root", c.RootDisk)
	} else {
		cmd.AddArg("--root", "first")
	}

	// Add the mapping to the virt-v2v, used mainly in the windows when migrating VMs with static IP
	if c.StaticIPs != "" {
		for _, mac := range strings.Split(c.StaticIPs, "_") {
			cmd.AddArg("--mac", mac)
		}
	}

	// Adds LUKS keys, if they exist
	if c.Luksdir != "" {
		err := utils.AddLUKSKeys(c.fileSystem, cmd, c.Luksdir)
		if err != nil {
			return fmt.Errorf("error adding LUKS keys: %v", err)
		}
	}
	if c.ExtraArgs != nil {
		cmd.AddExtraArgs(c.ExtraArgs...)
	}
	return nil
}

func (c *Conversion) RunVirtV2VInspection() error {
	v2vCmdBuilder := c.CommandBuilder.New("virt-v2v-inspector").
		AddFlag("-v").
		AddFlag("-x").
		AddArg("-if", "raw").
		AddArg("-i", "disk").
		AddArg("-O", c.InspectionOutputFile)
	err := c.addCommonArgs(v2vCmdBuilder)
	if err != nil {
		return err
	}
	for _, disk := range c.Disks {
		v2vCmdBuilder.AddPositional(disk.Link)
	}
	v2vCmd := v2vCmdBuilder.Build()
	v2vCmd.SetStdout(os.Stdout)
	v2vCmd.SetStderr(os.Stderr)
	return v2vCmd.Run()
}

func (c *Conversion) RunVirtV2vInPlace() error {
	v2vCmdBuilder := c.CommandBuilder.New("virt-v2v-in-place").
		AddFlag("-v").
		AddFlag("-x").
		AddArg("-i", "libvirtxml")
	err := c.addCommonArgs(v2vCmdBuilder)
	if err != nil {
		return err
	}
	v2vCmdBuilder.AddPositional(c.LibvirtDomainFile)

	v2vCmd := v2vCmdBuilder.Build()
	v2vCmd.SetStdout(os.Stdout)
	v2vCmd.SetStderr(os.Stderr)
	return v2vCmd.Run()
}

func (c *Conversion) addVirtV2vArgs(cmd utils.CommandBuilder) (err error) {
	cmd.AddFlag("-v").
		AddFlag("-x").
		AddArg("-o", "kubevirt").
		AddArg("-os", c.Workdir).
		// When converting VM with name that do not meet DNS1123 RFC requirements,
		// it should be changed to supported one to ensure the conversion does not fail.
		AddArg("-on", c.NewVmName)
	switch c.Source {
	case config.VSPHERE:
		err = c.addVirtV2vVsphereArgs(cmd)
		if err != nil {
			return err
		}
	case config.OVA:
		c.virtV2vOVAArgs(cmd)
	}
	return nil
}

func (c *Conversion) addVirtV2vVsphereArgs(cmd utils.CommandBuilder) (err error) {
	cmd.AddArg("-i", "libvirt").
		AddArg("-ic", c.LibvirtUrl).
		AddArg("-ip", c.SecretKey).
		AddArg("--hostname", c.HostName)

	err = c.addCommonArgs(cmd)
	if err != nil {
		return err
	}
	if info, err := os.Stat(c.VddkLibDir); err == nil && info.IsDir() {
		cmd.AddArg("-it", "vddk")
		cmd.AddArg("-io", fmt.Sprintf("vddk-libdir=%s", c.VddkLibDir))
		cmd.AddArg("-io", fmt.Sprintf("vddk-thumbprint=%s", c.Fingerprint))
		// Check if the config file exists but still allow the extra args to override the vddk-config for testing
		var extraArgs = c.ExtraArgs
		if _, err := os.Stat(c.VddkConfFile); !errors.Is(err, os.ErrNotExist) && len(extraArgs) == 0 {
			cmd.AddArg("-io", fmt.Sprintf("vddk-config=%s", c.VddkConfFile))
		}
	}
	cmd.AddPositional("--")
	cmd.AddPositional(c.VmName)
	return nil
}

func (c *Conversion) virtV2vOVAArgs(cmd utils.CommandBuilder) {
	cmd.AddArg("-i", "ova")
	cmd.AddPositional(c.DiskPath)
}

func (c *Conversion) RunVirtV2v() error {
	v2vCmdBuilder := c.CommandBuilder.New("virt-v2v")
	err := c.addVirtV2vArgs(v2vCmdBuilder)
	if err != nil {
		return err
	}

	v2vCmd := v2vCmdBuilder.Build()
	// The virt-v2v-monitor reads the virt-v2v stdout and processes it and exposes the progress of the migration.
	monitorCmd := c.CommandBuilder.New("/usr/local/bin/virt-v2v-monitor").Build()
	monitorCmd.SetStdout(os.Stdout)
	monitorCmd.SetStderr(os.Stderr)

	pipe, writer := io.Pipe()
	monitorCmd.SetStdin(pipe)
	v2vCmd.SetStdout(writer)
	v2vCmd.SetStderr(writer)
	defer writer.Close()

	if err := monitorCmd.Start(); err != nil {
		fmt.Printf("Error executing monitor command: %v\n", err)
		return err
	}
	if err := v2vCmd.Run(); err != nil {
		fmt.Printf("Error executing v2v command: %v\n", err)
		return err
	}

	// virt-v2v is done, we can close the pipe to virt-v2v-monitor
	writer.Close()

	if err := monitorCmd.Wait(); err != nil {
		fmt.Printf("Error waiting for virt-v2v-monitor to finish: %v\n", err)
		return err
	}

	return nil
}

func (c *Conversion) RunCustomize(osinfo utils.InspectionOS) error {
	var disks []string
	for _, disk := range c.Disks {
		disks = append(disks, disk.Link)
	}
	custom := customize.NewCustomize(c.AppConfig, disks, osinfo)
	return custom.Run()
}
