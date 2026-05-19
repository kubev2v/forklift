package probe

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/probe/parser"
)

// buildExtractionScript returns a guestfish script that cats network config files based on detected stacks.
func buildExtractionScript(guest *api.GuestInfo) string {
	var cmds []string

	if guest.OS.Family == api.OSFamilyLinux {
		cmds = append(cmds, "cat /etc/os-release")
	}

	cmds = append(cmds, buildIfcfgCommands(guest)...)
	cmds = append(cmds, buildNMCommands(guest)...)
	cmds = append(cmds, buildNetplanCommands(guest)...)
	cmds = append(cmds, buildInterfacesCommands(guest)...)
	cmds = append(cmds, buildWickedCommands(guest)...)

	if len(cmds) == 0 {
		return ""
	}
	return strings.Join(cmds, "\n") + "\n"
}

// buildIfcfgCommands returns guestfish commands to cat ifcfg files, or nil if not detected.
func buildIfcfgCommands(guest *api.GuestInfo) []string {
	if !guest.UsesIfcfg && !guest.UsesIfcfgSuse {
		return nil
	}
	cmds := []string{"echo ===IFCFG_START==="}
	if guest.UsesIfcfg {
		cmds = append(cmds, "-glob cat /etc/sysconfig/network-scripts/ifcfg-*")
	}
	if guest.UsesIfcfgSuse {
		cmds = append(cmds, "-glob cat /etc/sysconfig/network/ifcfg-*")
	}
	return append(cmds, "echo ===IFCFG_END===")
}

// buildNMCommands returns guestfish commands to cat NetworkManager connections, or nil if not detected.
func buildNMCommands(guest *api.GuestInfo) []string {
	if !guest.UsesNetworkManager {
		return nil
	}
	return []string{
		"echo ===NM_START===",
		"-glob cat /etc/NetworkManager/system-connections/*",
		"echo ===NM_END===",
	}
}

// buildNetplanCommands returns guestfish commands to cat netplan YAML files, or nil if not detected.
func buildNetplanCommands(guest *api.GuestInfo) []string {
	if !guest.UsesNetplan {
		return nil
	}
	return []string{
		"echo ===NETPLAN_START===",
		"-glob cat /etc/netplan/*.yaml",
		"-glob cat /etc/netplan/*.yml",
		"echo ===NETPLAN_END===",
	}
}

// buildInterfacesCommands returns guestfish commands to cat /etc/network/interfaces, or nil if not detected.
func buildInterfacesCommands(guest *api.GuestInfo) []string {
	if !guest.UsesIfquery && !guest.UsesInterfacesD {
		return nil
	}
	cmds := []string{"echo ===INTERFACES_START==="}
	if guest.UsesIfquery {
		cmds = append(cmds, "-cat /etc/network/interfaces")
	}
	if guest.UsesInterfacesD {
		cmds = append(cmds, "-glob cat /etc/network/interfaces.d/*")
	}
	return append(cmds, "echo ===INTERFACES_END===")
}

// buildWickedCommands returns guestfish commands to list and cat wicked lease files, or nil if not detected.
func buildWickedCommands(guest *api.GuestInfo) []string {
	if !guest.UsesWicked {
		return nil
	}
	return []string{
		"echo ===WICKED_START===",
		"-ls /var/lib/wicked/",
		"echo ===WICKED_XML===",
		"-glob cat /var/lib/wicked/lease-*",
		"echo ===WICKED_END===",
	}
}

// parseExtraction splits guestfish output by section markers and delegates to per-stack parsers.
func parseExtraction(output string, guest *api.GuestInfo) error {
	if err := parseOsRelease(output, guest); err != nil {
		return fmt.Errorf("parsing os-release: %w", err)
	}

	if section := extractSection(output, "===IFCFG_START===", "===IFCFG_END==="); section != "" {
		ifaces, err := parser.Ifcfg(section)
		if err != nil {
			return fmt.Errorf("parsing ifcfg: %w", err)
		}
		guest.Interfaces = append(guest.Interfaces, ifaces...)
	}
	if section := extractSection(output, "===NM_START===", "===NM_END==="); section != "" {
		ifaces, err := parser.NM(section)
		if err != nil {
			return fmt.Errorf("parsing NM: %w", err)
		}
		guest.Interfaces = append(guest.Interfaces, ifaces...)
	}
	if section := extractSection(output, "===NETPLAN_START===", "===NETPLAN_END==="); section != "" {
		result, err := parser.Netplan(section)
		if err != nil {
			return fmt.Errorf("parsing netplan: %w", err)
		}
		guest.Interfaces = append(guest.Interfaces, result.Interfaces...)
		if result.Renderer != "" {
			guest.NetplanRenderer = result.Renderer
		}
	}
	if section := extractSection(output, "===INTERFACES_START===", "===INTERFACES_END==="); section != "" {
		ifaces, err := parser.Interfaces(section)
		if err != nil {
			return fmt.Errorf("parsing interfaces: %w", err)
		}
		guest.Interfaces = append(guest.Interfaces, ifaces...)
	}
	wickedFiles := extractSection(output, "===WICKED_START===", "===WICKED_XML===")
	wickedXML := extractSection(output, "===WICKED_XML===", "===WICKED_END===")
	if wickedFiles != "" || wickedXML != "" {
		ifaces, err := parser.Wicked(wickedFiles, wickedXML)
		if err != nil {
			return fmt.Errorf("parsing wicked: %w", err)
		}
		guest.Interfaces = append(guest.Interfaces, ifaces...)
	}
	return nil
}

// extractSection returns the text between startMarker and endMarker, or "" if not found.
func extractSection(output, startMarker, endMarker string) string {
	startIdx := strings.Index(output, startMarker)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(startMarker)
	if startIdx < len(output) && output[startIdx] == '\n' {
		startIdx++
	}
	if startIdx >= len(output) {
		return ""
	}
	endIdx := strings.Index(output[startIdx:], endMarker)
	if endIdx == -1 {
		return output[startIdx:]
	}
	return output[startIdx : startIdx+endIdx]
}

// parseOsRelease extracts ID and VERSION_ID from /etc/os-release output into guest.OS.
func parseOsRelease(output string, guest *api.GuestInfo) error {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			guest.OS.Distro = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			guest.OS.Version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
		if strings.HasPrefix(line, "===") {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning os-release output: %w", err)
	}
	return nil
}
