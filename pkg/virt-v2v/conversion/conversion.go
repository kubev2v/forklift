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
	// Ensure disk files are ready for conversion.
	// virt-v2v-in-place reads from vSphere (via libvirt connection) and writes RAW format
	// directly to the disk files specified in the libvirt XML. If the files already exist
	// with invalid/corrupted VMDK data (e.g., from a previous failed attempt), virt-v2v-in-place
	// or qemu-img (used internally) may fail when trying to process them. We need to ensure
	// the files are empty or don't exist so virt-v2v-in-place can write RAW format to them.
	err := c.prepareDiskFilesForInPlace()
	if err != nil {
		return fmt.Errorf("failed to prepare disk files for in-place conversion: %v", err)
	}

	v2vCmdBuilder := c.CommandBuilder.New("virt-v2v-in-place").
		AddFlag("-v").
		AddFlag("-x").
		AddArg("-i", "libvirtxml")
	err = c.addCommonArgs(v2vCmdBuilder)
	if err != nil {
		return err
	}
	v2vCmdBuilder.AddPositional(c.LibvirtDomainFile)

	v2vCmd := v2vCmdBuilder.Build()
	v2vCmd.SetStdout(os.Stdout)
	v2vCmd.SetStderr(os.Stderr)
	return v2vCmd.Run()
}

// prepareDiskFilesForInPlace ensures disk files are ready for virt-v2v-in-place.
// virt-v2v-in-place reads VM data from vSphere (via libvirt connection) and writes
// RAW format directly to the disk files specified in the libvirt XML. The files should
// be empty or not exist - if they contain invalid/corrupted VMDK data from a previous
// failed attempt, virt-v2v-in-place or qemu-img (used internally) may fail when trying
// to detect or process the file format.
func (c *Conversion) prepareDiskFilesForInPlace() error {
	for _, disk := range c.Disks {
		if disk.IsBlockDev {
			// Block devices don't need preparation
			continue
		}

		// Check if file exists
		info, err := os.Stat(disk.Path)
		if err != nil {
			if os.IsNotExist(err) {
				// File doesn't exist - that's fine, virt-v2v-in-place will create it
				continue
			}
			return fmt.Errorf("failed to stat disk file %s: %v", disk.Path, err)
		}

		// File exists - check if it's empty or has invalid content
		if info.Size() > 0 {
			// Try to detect if it's a valid image format
			// If it's corrupted VMDK or invalid data, truncate it
			// Open the file and check first few bytes
			file, err := os.Open(disk.Path)
			if err != nil {
				return fmt.Errorf("failed to open disk file %s: %v", disk.Path, err)
			}
			defer file.Close()

			// Read first 512 bytes to check format
			buf := make([]byte, 512)
			n, err := file.Read(buf)
			if err != nil && err != io.EOF {
				return fmt.Errorf("failed to read disk file %s: %v", disk.Path, err)
			}

			// Check if it looks like VMDK (VMDK descriptor starts with "# Disk DescriptorFile")
			// or if it's corrupted/invalid data
			isVMDK := false
			if n >= 20 {
				header := string(buf[:min(20, n)])
				if len(header) >= 20 && header[:20] == "# Disk DescriptorFile" {
					isVMDK = true
				}
			}

			// If it's VMDK or we can't determine the format, truncate it
			// virt-v2v-in-place will write RAW format directly
			if isVMDK || n < 512 {
				fmt.Printf("Truncating disk file %s (size: %d) to prepare for in-place conversion\n", disk.Path, info.Size())
				err = os.Truncate(disk.Path, 0)
				if err != nil {
					return fmt.Errorf("failed to truncate disk file %s: %v", disk.Path, err)
				}
			}
			// If it's already RAW format and valid, leave it as is
		}
		// If file is empty (size 0), that's perfect - virt-v2v-in-place will write to it
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
