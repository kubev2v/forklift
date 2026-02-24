package conversion

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customize"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
	"libvirt.org/go/libvirt"
	libvirtxml "libvirt.org/go/libvirtxml"
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

// addCommonArgs adds v2v arguments which are shared between all virt-v2v commands
// (virt-v2v, virt-v2v-in-place, and virt-v2v-inspector)
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
	if c.NbdeClevis {
		cmd.AddArgs("--key", "all:clevis")
	} else if c.Luksdir != "" {
		// Adds LUKS keys, if they exist
		err := utils.AddLUKSKeys(c.fileSystem, cmd, c.Luksdir)
		if err != nil {
			return fmt.Errorf("error adding LUKS keys: %v", err)
		}
	}
	return nil
}

// addConversionExtraArgs adds extra args that apply ONLY to virt-v2v and virt-v2v-in-place
func (c *Conversion) addConversionExtraArgs(cmd utils.CommandBuilder) {
	if c.ExtraArgs != nil {
		cmd.AddExtraArgs(c.ExtraArgs...)
	}
}

// addInspectorExtraArgs adds extra args that apply ONLY to virt-v2v-inspector
func (c *Conversion) addInspectorExtraArgs(cmd utils.CommandBuilder) {
	if c.InspectorExtraArgs != nil {
		cmd.AddExtraArgs(c.InspectorExtraArgs...)
	}
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
	c.addInspectorExtraArgs(v2vCmdBuilder)
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
	c.addConversionExtraArgs(v2vCmdBuilder)
	v2vCmdBuilder.AddPositional(c.LibvirtDomainFile)
	v2vCmd := v2vCmdBuilder.Build()
	v2vCmd.SetStdout(os.Stdout)
	v2vCmd.SetStderr(os.Stderr)
	return v2vCmd.Run()
}

// RunVirtV2vInPlaceDisk runs virt-v2v-in-place using disk mode (-i disk).
// This is used for providers like EC2 that don't have libvirt and where
// the disks are already populated and mounted as block devices or files.
func (c *Conversion) RunVirtV2vInPlaceDisk() error {
	if len(c.Disks) == 0 {
		return fmt.Errorf("no disks found for in-place conversion")
	}

	v2vCmdBuilder := c.CommandBuilder.New("virt-v2v-in-place").
		AddFlag("-v").
		AddFlag("-x").
		AddArg("-i", "disk")

	err := c.addCommonArgs(v2vCmdBuilder)
	if err != nil {
		return err
	}
	c.addConversionExtraArgs(v2vCmdBuilder)

	// Add all disks as positional arguments
	for _, disk := range c.Disks {
		v2vCmdBuilder.AddPositional(disk.Link)
	}

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
	c.addConversionExtraArgs(cmd)
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

// addVirtV2vVsphereArgsForInspection adds vSphere-specific args WITHOUT conversion extra args
// This is used for remote inspection where we want inspector-specific args instead
func (c *Conversion) addVirtV2vVsphereArgsForInspection(cmd utils.CommandBuilder) (err error) {
	cmd.AddArg("-i", "libvirt").
		AddArg("-ic", c.LibvirtUrl).
		AddArg("-ip", c.SecretKey).
		AddArg("--hostname", c.HostName)

	err = c.addCommonArgs(cmd)
	if err != nil {
		return err
	}
	// Note: NO addConversionExtraArgs here - this is for inspection
	if info, err := os.Stat(c.VddkLibDir); err == nil && info.IsDir() {
		cmd.AddArg("-it", "vddk")
		cmd.AddArg("-io", fmt.Sprintf("vddk-libdir=%s", c.VddkLibDir))
		cmd.AddArg("-io", fmt.Sprintf("vddk-thumbprint=%s", c.Fingerprint))
		// Always use vddk-config for inspection if it exists (no extra args override)
		if _, err := os.Stat(c.VddkConfFile); !errors.Is(err, os.ErrNotExist) {
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

func (c *Conversion) RunRemoteV2vInspection() (err error) {
	v2vCmdBuilder := c.CommandBuilder.New("virt-v2v-inspector").
		AddFlag("-v").
		AddFlag("-x")

	err = c.addVirtV2vRemoteInspectionArgs(v2vCmdBuilder)
	if err != nil {
		return err
	}

	// Use the inspection-specific helper that doesn't add conversion extra args
	err = c.addVirtV2vVsphereArgsForInspection(v2vCmdBuilder)
	if err != nil {
		return err
	}
	c.addInspectorExtraArgs(v2vCmdBuilder)

	v2vCmd := v2vCmdBuilder.Build()
	v2vCmd.SetStdout(os.Stdout)
	v2vCmd.SetStderr(os.Stderr)
	return v2vCmd.Run()
}

func (c *Conversion) addVirtV2vRemoteInspectionArgs(cmd utils.CommandBuilder) (err error) {
	if len(c.RemoteInspectionDisks) == 0 {
		return fmt.Errorf("No remote disks were supplied")
	}
	for _, disk := range c.RemoteInspectionDisks {
		cmd.AddArg("-io", fmt.Sprintf("vddk-file=%s", disk))
	}
	return
}

// retrieve and modify the domain XML from libvirt
func (c *Conversion) GetDomainXML() (string, error) {
	libvirtURL, err := url.Parse(c.LibvirtUrl)

	if err != nil {
		return "", fmt.Errorf("failed to parse libvirt URL: %w", err)
	}

	usernameData, err := os.ReadFile(c.AccessKeyId)
	if err != nil {
		return "", fmt.Errorf("failed to read username from secret: %w", err)
	}
	username := string(usernameData)

	passwordData, err := os.ReadFile(c.SecretKey)
	if err != nil {
		return "", fmt.Errorf("failed to read password from secret: %w", err)
	}
	password := string(passwordData)

	auth := &libvirt.ConnectAuth{
		CredType: []libvirt.ConnectCredentialType{
			libvirt.CRED_AUTHNAME,
			libvirt.CRED_PASSPHRASE,
		},
		Callback: func(creds []*libvirt.ConnectCredential) {
			for _, cred := range creds {
				switch cred.Type {
				case libvirt.CRED_AUTHNAME:
					cred.Result = username
					cred.ResultLen = len(username)
				case libvirt.CRED_PASSPHRASE:
					cred.Result = password
					cred.ResultLen = len(password)
				}
			}
		},
	}

	conn, err := libvirt.NewConnectWithAuth(libvirtURL.String(), auth, 0)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	domain, err := conn.LookupDomainByName(c.VmName)
	if err != nil {
		return "", fmt.Errorf("failed to lookup domain %s: %w", c.VmName, err)
	}
	defer func() {
		if err := domain.Free(); err != nil {
			fmt.Printf("Failed to free libvirt domain: %s", err)
		}
	}()

	domainXML, err := domain.GetXMLDesc(0)
	if err != nil {
		return "", fmt.Errorf("failed to get domain XML: %w", err)
	}

	modifiedXML, err := c.updateDiskPaths(domainXML)
	if err != nil {
		return "", fmt.Errorf("failed to update disk paths in domain XML: %w", err)
	}

	return modifiedXML, nil
}

func updateDiskSource(disk *libvirtxml.DomainDisk, path string) bool {
	if disk.Source == nil {
		return false
	}

	switch {
	case disk.Source.File != nil:
		disk.Source.File.File = path
	case disk.Source.Block != nil:
		disk.Source.Block.Dev = path
	default:
		return false
	}
	return true
}

// modify the domain XML to use the local disk paths for in-place conversions
func (c *Conversion) updateDiskPaths(domainXML string) (string, error) {
	fmt.Printf("Updating disk paths: found %d disks\n", len(c.Disks))
	domain := &libvirtxml.Domain{}
	err := domain.Unmarshal(domainXML)
	if err != nil {
		return "", fmt.Errorf("failed to parse domain XML: %w", err)
	}
	updatedDisks := []libvirtxml.DomainDisk{}
	diskIdx := 0
	for _, disk := range domain.Devices.Disks {
		if diskIdx >= len(c.Disks) {
			fmt.Printf("WARNING: disk %d in domain XML but only %d disks available\n", diskIdx, len(c.Disks))
			break
		}
		if disk.Device == "cdrom" {
			continue
		}
		newPath := c.Disks[diskIdx].Link
		if updated := updateDiskSource(&disk, newPath); updated {
			updatedDisks = append(updatedDisks, disk)
		}
		diskIdx++
	}
	domain.Devices.Disks = updatedDisks

	modifiedXML, err := domain.Marshal()
	if err != nil {
		return "", fmt.Errorf("failed to marshal modified domain XML: %w", err)
	}

	return modifiedXML, nil
}
