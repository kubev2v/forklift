package probe

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// detect uses direct GuestHandle calls to probe for OS type and network stacks.
func detect(g api.GuestHandle, guest *api.GuestInfo) error {
	isWindows, err := g.IsDir("/Windows/System32")
	if err != nil {
		return fmt.Errorf("checking /Windows/System32: %w", err)
	}
	if isWindows {
		guest.OS.Family = api.OSFamilyWindows
		return nil
	}

	hasOsRelease, err := g.IsFile("/etc/os-release")
	if err != nil {
		return fmt.Errorf("checking /etc/os-release: %w", err)
	}
	if hasOsRelease {
		guest.OS.Family = api.OSFamilyLinux
	} else {
		guest.OS.Family = api.OSFamilyUnknown
	}

	guest.UsesIfcfg, _ = g.IsDir("/etc/sysconfig/network-scripts")
	guest.UsesIfcfgSuse, _ = g.IsDir("/etc/sysconfig/network")
	guest.UsesNetworkManager, _ = g.IsDir("/etc/NetworkManager/system-connections")
	guest.UsesNetplan, _ = g.IsDir("/etc/netplan")
	guest.UsesIfquery, _ = g.IsFile("/etc/network/interfaces")
	guest.UsesInterfacesD, _ = g.IsDir("/etc/network/interfaces.d")
	wickedEtc, _ := g.IsDir("/etc/wicked")
	wickedVar, _ := g.IsDir("/var/lib/wicked")
	guest.UsesWicked = wickedEtc || wickedVar
	guest.UsesNMDhcpLease, _ = g.IsDir("/var/lib/NetworkManager")
	guest.UsesDhclient, _ = g.IsDir("/var/lib/dhclient")

	// Cloud-init
	hasCloudDir, _ := g.IsDir("/etc/cloud")
	hasCloudBin, _ := g.IsFile("/usr/bin/cloud-init")
	guest.CloudInit.Present = hasCloudDir || hasCloudBin
	guest.CloudInit.HasCloudCfg, _ = g.IsFile("/etc/cloud/cloud.cfg")
	guest.CloudInit.HasCloudCfgD, _ = g.IsDir("/etc/cloud/cloud.cfg.d")
	guest.CloudInit.HasInstanceData, _ = g.IsDir("/var/lib/cloud/instance")
	guest.CloudInit.HasSeedData, _ = g.IsDir("/var/lib/cloud/seed")

	// SSH
	guest.SSH.Present, _ = g.IsFile("/usr/sbin/sshd")
	guest.SSH.HasConfig, _ = g.IsFile("/etc/ssh/sshd_config")
	guest.SSH.HasRootAuthorizedKeys, _ = g.IsFile("/root/.ssh/authorized_keys")
	hostKeys, _ := g.GlobExpand("/etc/ssh/ssh_host_*_key")
	guest.SSH.HasHostKeys = len(hostKeys) > 0

	// Console
	guest.Console.HasGrubDefaults, _ = g.IsFile("/etc/default/grub")

	return nil
}

// parseOsRelease extracts ID and VERSION_ID from /etc/os-release content.
func parseOsRelease(content string, guest *api.GuestInfo) error {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			guest.OS.Distro = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			guest.OS.Version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}
	return scanner.Err()
}
