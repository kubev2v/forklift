package console

import (
	"bufio"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// ParseGrubDefaults parses /etc/default/grub and extracts console= parameters
// from GRUB_CMDLINE_LINUX and GRUB_CMDLINE_LINUX_DEFAULT.
func ParseGrubDefaults(section string) []api.ConsoleDevice {
	var consoles []api.ConsoleDevice

	scanner := bufio.NewScanner(strings.NewReader(section))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := kv[0]
		if key != "GRUB_CMDLINE_LINUX" && key != "GRUB_CMDLINE_LINUX_DEFAULT" {
			continue
		}

		val := stripShellQuotes(kv[1])
		consoles = append(consoles, parseConsoleParams(val)...)
	}

	return consoles
}

// ParseSerialGettyUnits extracts device names from serial-getty unit filenames.
// Input: directory listing entries like "serial-getty@ttyS0.service".
// Returns: ["ttyS0"]
func ParseSerialGettyUnits(entries []string) []string {
	var devices []string
	for _, name := range entries {
		if !strings.HasPrefix(name, "serial-getty@") {
			continue
		}
		dev := strings.TrimPrefix(name, "serial-getty@")
		dev = strings.TrimSuffix(dev, ".service")
		if dev != "" {
			devices = append(devices, dev)
		}
	}
	return devices
}

// parseConsoleParams extracts console= parameters from a kernel cmdline string.
// Example: "console=ttyS0,115200n8 console=tty0 quiet"
// Returns: [{Device:"ttyS0", Baud:"115200n8"}, {Device:"tty0", Baud:""}]
func parseConsoleParams(cmdline string) []api.ConsoleDevice {
	var consoles []api.ConsoleDevice
	for _, token := range strings.Fields(cmdline) {
		if !strings.HasPrefix(token, "console=") {
			continue
		}
		val := strings.TrimPrefix(token, "console=")
		parts := strings.SplitN(val, ",", 2)
		dev := api.ConsoleDevice{Device: parts[0]}
		if len(parts) > 1 {
			dev.Baud = parts[1]
		}
		consoles = append(consoles, dev)
	}
	return consoles
}

// stripShellQuotes removes surrounding double or single quotes.
func stripShellQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
