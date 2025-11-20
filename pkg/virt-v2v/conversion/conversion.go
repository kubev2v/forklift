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
	if c.NbdeClevis {
		cmd.AddArgs("--key", "all:clevis")
	} else if c.Luksdir != "" {
		// Adds LUKS keys, if they exist
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

func (c *Conversion) RunRemoteV2vInspection() (err error) {
	v2vCmdBuilder := c.CommandBuilder.New("virt-v2v-inspector").
		AddFlag("-v").
		AddFlag("-x")

	err = c.addVirtV2vRemoteInspectionArgs(v2vCmdBuilder)
	if err != nil {
		return err
	}

	err = c.addVirtV2vVsphereArgs(v2vCmdBuilder)
	if err != nil {
		return err
	}

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

// modify the domain XML to use the local disk paths for in-place conversions
func (c *Conversion) updateDiskPaths(domainXML string) (string, error) {
	fmt.Printf("Updating disk paths: found %d disks\n", len(c.Disks))
	domain := &libvirtxml.Domain{}
	err := domain.Unmarshal(domainXML)
	if err != nil {
		return "", fmt.Errorf("failed to parse domain XML: %w", err)
	}

	for i, domainDisk := range domain.Devices.Disks {
		if i >= len(c.Disks) {
			fmt.Printf("WARNING: disk %d in domain XML but only %d disks available\n", i, len(c.Disks))
			continue
		}

		if domainDisk.Source == nil {
			fmt.Printf("skipping disk %d: no source defined\n", i)
			continue
		}

		if domainDisk.Source.File != nil {
			domainDisk.Source.File.File = c.Disks[i].Link
			fmt.Printf("Updated disk %d file source to: %s\n", i, c.Disks[i].Link)
		} else if domainDisk.Source.Block != nil {
			domainDisk.Source.Block.Dev = c.Disks[i].Link
			fmt.Printf("Updated disk %d block source to: %s\n", i, c.Disks[i].Link)
		} else {
			fmt.Printf("WARNING: skipping disk %d: unsupported source type\n", i)
		}
	}

	modifiedXML, err := domain.Marshal()
	if err != nil {
		return "", fmt.Errorf("failed to marshal modified domain XML: %w", err)
	}

	return modifiedXML, nil
}
