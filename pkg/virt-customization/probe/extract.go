package probe

import (
	"fmt"
	"path"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
	"github.com/kubev2v/forklift/pkg/virt-customization/probe/cloudinit"
	"github.com/kubev2v/forklift/pkg/virt-customization/probe/console"
	"github.com/kubev2v/forklift/pkg/virt-customization/probe/netcfg"
	"github.com/kubev2v/forklift/pkg/virt-customization/probe/ssh"
)

// extract reads network configuration files from the guest via direct
// GuestHandle API calls and populates guest.Interfaces.
func extract(g api.GuestHandle, guest *api.GuestInfo) error {
	if guest.OS.Family == api.OSFamilyLinux {
		if err := extractOsRelease(g, guest); err != nil {
			return fmt.Errorf("os-release: %w", err)
		}
	}

	if guest.UsesIfcfg || guest.UsesIfcfgSuse {
		if err := extractIfcfg(g, guest); err != nil {
			return fmt.Errorf("ifcfg: %w", err)
		}
	}
	if guest.UsesNetworkManager {
		if err := extractNM(g, guest); err != nil {
			return fmt.Errorf("NM: %w", err)
		}
	}
	if guest.UsesNetplan {
		if err := extractNetplan(g, guest); err != nil {
			return fmt.Errorf("netplan: %w", err)
		}
	}
	if guest.UsesIfquery || guest.UsesInterfacesD {
		if err := extractInterfaces(g, guest); err != nil {
			return fmt.Errorf("interfaces: %w", err)
		}
	}
	if guest.UsesWicked {
		if err := extractWicked(g, guest); err != nil {
			return fmt.Errorf("wicked: %w", err)
		}
	}
	if guest.UsesNMDhcpLease {
		if err := extractNMDhcpLease(g, guest); err != nil {
			return fmt.Errorf("NM DHCP lease: %w", err)
		}
	}
	if guest.UsesDhclient {
		if err := extractDhclient(g, guest); err != nil {
			return fmt.Errorf("dhclient: %w", err)
		}
	}

	if guest.CloudInit.Present {
		if err := extractCloudInit(g, guest); err != nil {
			return fmt.Errorf("cloud-init: %w", err)
		}
	}
	if guest.SSH.HasConfig {
		if err := extractSSH(g, guest); err != nil {
			return fmt.Errorf("SSH: %w", err)
		}
	}
	if guest.Console.HasGrubDefaults {
		if err := extractConsole(g, guest); err != nil {
			return fmt.Errorf("console: %w", err)
		}
	}

	return nil
}

func extractOsRelease(g api.GuestHandle, guest *api.GuestInfo) error {
	content, err := g.Cat("/etc/os-release")
	if err != nil {
		return err
	}
	return parseOsRelease(content, guest)
}

func extractIfcfg(g api.GuestHandle, guest *api.GuestInfo) error {
	var combined strings.Builder
	if guest.UsesIfcfg {
		if err := catGlob(g, "/etc/sysconfig/network-scripts/ifcfg-*", &combined); err != nil {
			return err
		}
	}
	if guest.UsesIfcfgSuse {
		if err := catGlob(g, "/etc/sysconfig/network/ifcfg-*", &combined); err != nil {
			return err
		}
	}
	if combined.Len() == 0 {
		return nil
	}
	ifaces, err := netcfg.Ifcfg(combined.String())
	if err != nil {
		return err
	}
	guest.Interfaces = append(guest.Interfaces, ifaces...)
	return nil
}

func extractNM(g api.GuestHandle, guest *api.GuestInfo) error {
	var combined strings.Builder
	if err := catGlob(g, "/etc/NetworkManager/system-connections/*", &combined); err != nil {
		return err
	}
	if combined.Len() == 0 {
		return nil
	}
	ifaces, err := netcfg.NM(combined.String())
	if err != nil {
		return err
	}
	guest.Interfaces = append(guest.Interfaces, ifaces...)
	return nil
}

func extractNetplan(g api.GuestHandle, guest *api.GuestInfo) error {
	var combined strings.Builder
	if err := catGlob(g, "/etc/netplan/*.yaml", &combined); err != nil {
		return err
	}
	if err := catGlob(g, "/etc/netplan/*.yml", &combined); err != nil {
		return err
	}
	if combined.Len() == 0 {
		return nil
	}
	result, err := netcfg.Netplan(combined.String())
	if err != nil {
		return err
	}
	guest.Interfaces = append(guest.Interfaces, result.Interfaces...)
	if result.Renderer != "" {
		guest.NetplanRenderer = result.Renderer
	}
	return nil
}

func extractInterfaces(g api.GuestHandle, guest *api.GuestInfo) error {
	var combined strings.Builder
	if guest.UsesIfquery {
		content, err := g.Cat("/etc/network/interfaces")
		if err != nil {
			fmt.Printf("warning: reading /etc/network/interfaces: %v\n", err)
		} else {
			combined.WriteString(content)
			combined.WriteByte('\n')
		}
	}
	if guest.UsesInterfacesD {
		if err := catGlob(g, "/etc/network/interfaces.d/*", &combined); err != nil {
			return err
		}
	}
	if combined.Len() == 0 {
		return nil
	}
	ifaces, err := netcfg.Interfaces(combined.String())
	if err != nil {
		return err
	}
	guest.Interfaces = append(guest.Interfaces, ifaces...)
	return nil
}

func extractWicked(g api.GuestHandle, guest *api.GuestInfo) error {
	var filesList string
	files, err := g.Ls("/var/lib/wicked/")
	if err != nil {
		fmt.Printf("warning: listing /var/lib/wicked/: %v\n", err)
	} else {
		filesList = strings.Join(files, "\n")
	}

	var xmlContent strings.Builder
	if err := catGlob(g, "/var/lib/wicked/lease-*", &xmlContent); err != nil {
		return err
	}

	if filesList == "" && xmlContent.Len() == 0 {
		return nil
	}

	ifaces, err := netcfg.Wicked(filesList, xmlContent.String())
	if err != nil {
		return err
	}
	guest.Interfaces = append(guest.Interfaces, ifaces...)
	return nil
}

func extractNMDhcpLease(g api.GuestHandle, guest *api.GuestInfo) error {
	var filesList string
	files, err := g.Ls("/var/lib/NetworkManager/")
	if err != nil {
		fmt.Printf("warning: listing /var/lib/NetworkManager/: %v\n", err)
	} else {
		filesList = strings.Join(files, "\n")
	}

	var leaseContent strings.Builder
	if err := catGlob(g, "/var/lib/NetworkManager/*.lease", &leaseContent); err != nil {
		return err
	}

	var timestampsContent string
	ts, err := g.Cat("/var/lib/NetworkManager/timestamps")
	if err != nil {
		fmt.Printf("warning: reading /var/lib/NetworkManager/timestamps: %v\n", err)
	} else {
		timestampsContent = ts
	}

	if filesList == "" && leaseContent.Len() == 0 {
		return nil
	}

	ifaces, err := netcfg.NMDhcpLease(filesList, leaseContent.String(), timestampsContent)
	if err != nil {
		return err
	}
	guest.Interfaces = append(guest.Interfaces, ifaces...)
	return nil
}

func extractDhclient(g api.GuestHandle, guest *api.GuestInfo) error {
	var combined strings.Builder
	if err := catGlob(g, "/var/lib/dhclient/dhclient-*", &combined); err != nil {
		return err
	}
	// Also check NM dhclient files
	if guest.UsesNMDhcpLease {
		if err := catGlob(g, "/var/lib/NetworkManager/dhclient-*", &combined); err != nil {
			return err
		}
	}
	if combined.Len() == 0 {
		return nil
	}
	ifaces, err := netcfg.Dhclient(combined.String())
	if err != nil {
		return err
	}
	guest.Interfaces = append(guest.Interfaces, ifaces...)
	return nil
}

func extractCloudInit(g api.GuestHandle, guest *api.GuestInfo) error {
	// Parse cloud.cfg + cloud.cfg.d/*.cfg
	if guest.CloudInit.HasCloudCfg || guest.CloudInit.HasCloudCfgD {
		var combined strings.Builder
		if guest.CloudInit.HasCloudCfg {
			content, err := g.Cat("/etc/cloud/cloud.cfg")
			if err != nil {
				fmt.Printf("warning: reading /etc/cloud/cloud.cfg: %v\n", err)
			} else {
				combined.WriteString(content)
				combined.WriteByte('\n')
			}
		}
		if guest.CloudInit.HasCloudCfgD {
			if err := catGlob(g, "/etc/cloud/cloud.cfg.d/*.cfg", &combined); err != nil {
				return err
			}
		}
		if combined.Len() > 0 {
			result, err := cloudinit.ParseCloudCfg(combined.String())
			if err != nil {
				fmt.Printf("warning: parsing cloud.cfg: %v\n", err)
			}
			guest.CloudInit.DatasourceList = result.DatasourceList
			guest.CloudInit.NetworkConfigDisabled = result.NetworkConfigDisabled
		}
	}

	// Read active datasource from instance state
	if guest.CloudInit.HasInstanceData {
		content, err := g.Cat("/var/lib/cloud/instance/datasource")
		if err != nil {
			fmt.Printf("warning: reading cloud-init datasource: %v\n", err)
		} else {
			guest.CloudInit.ActiveDatasource = cloudinit.ParseDatasourceFile(content)
		}

		idContent, err := g.Cat("/var/lib/cloud/instance/instance-id")
		if err != nil {
			fmt.Printf("warning: reading cloud-init instance-id: %v\n", err)
		} else {
			guest.CloudInit.InstanceID = strings.TrimSpace(idContent)
		}
	}

	return nil
}

func extractSSH(g api.GuestHandle, guest *api.GuestInfo) error {
	var combined strings.Builder
	content, err := g.Cat("/etc/ssh/sshd_config")
	if err != nil {
		return fmt.Errorf("reading sshd_config: %w", err)
	}
	combined.WriteString(content)
	combined.WriteByte('\n')

	// Include drop-ins (sshd_config.d/*.conf)
	if err := catGlob(g, "/etc/ssh/sshd_config.d/*.conf", &combined); err != nil {
		return err
	}

	result := ssh.ParseSSHDConfig(combined.String())
	guest.SSH.PermitRootLogin = result.PermitRootLogin
	guest.SSH.PasswordAuthentication = result.PasswordAuthentication
	guest.SSH.Port = result.Port

	// Host key types from glob filenames
	hostKeys, _ := g.GlobExpand("/etc/ssh/ssh_host_*_key")
	for _, keyPath := range hostKeys {
		keyType := ssh.HostKeyTypeFromFilename(path.Base(keyPath))
		if keyType != "" {
			guest.SSH.HostKeyTypes = append(guest.SSH.HostKeyTypes, keyType)
		}
	}

	// Service enabled check via systemd wants directory
	entries, err := g.Ls("/etc/systemd/system/multi-user.target.wants/")
	if err == nil {
		for _, entry := range entries {
			if entry == "sshd.service" || entry == "ssh.service" {
				guest.SSH.ServiceEnabled = true
				break
			}
		}
	}

	return nil
}

func extractConsole(g api.GuestHandle, guest *api.GuestInfo) error {
	content, err := g.Cat("/etc/default/grub")
	if err != nil {
		return fmt.Errorf("reading /etc/default/grub: %w", err)
	}
	guest.Console.SerialConsoles = console.ParseGrubDefaults(content)

	// Serial getty units
	entries, err := g.Ls("/etc/systemd/system/getty.target.wants/")
	if err != nil {
		fmt.Printf("warning: listing getty.target.wants: %v\n", err)
	} else {
		guest.Console.SerialGettyDevices = console.ParseSerialGettyUnits(entries)
		guest.Console.HasSerialGetty = len(guest.Console.SerialGettyDevices) > 0
	}

	return nil
}

// catGlob expands a guest glob pattern and concatenates the contents
// of all matching files into dst.
func catGlob(g api.GuestHandle, pattern string, dst *strings.Builder) error {
	matches, _ := g.GlobExpand(pattern) // glob failure is non-fatal (directory may exist but be empty)
	for _, path := range matches {
		content, err := g.Cat(path)
		if err != nil {
			fmt.Printf("warning: reading %s: %v\n", path, err)
			continue
		}
		dst.WriteString(content)
		dst.WriteByte('\n')
	}
	return nil
}
