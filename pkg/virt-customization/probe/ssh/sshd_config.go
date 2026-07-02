package ssh

import (
	"bufio"
	"strconv"
	"strings"
)

// SSHDConfigResult holds the fields extracted from sshd_config.
type SSHDConfigResult struct {
	PermitRootLogin        string
	PasswordAuthentication *bool
	Port                   int
}

// ParseSSHDConfig parses concatenated sshd_config + sshd_config.d/*.conf
// content and extracts the directives needed for login capability detection.
//
// sshd_config uses "first match wins" semantics for most directives, but
// Include'd files are processed inline. Since we concatenate main config
// first, then drop-ins, the first occurrence of each directive wins.
func ParseSSHDConfig(section string) SSHDConfigResult {
	var result SSHDConfigResult
	gotPermitRoot := false
	gotPassAuth := false
	gotPort := false

	scanner := bufio.NewScanner(strings.NewReader(section))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.ToLower(fields[0])
		val := fields[1]

		switch key {
		case "permitrootlogin":
			if !gotPermitRoot {
				result.PermitRootLogin = val
				gotPermitRoot = true
			}
		case "passwordauthentication":
			if !gotPassAuth {
				b := strings.EqualFold(val, "yes")
				result.PasswordAuthentication = &b
				gotPassAuth = true
			}
		case "port":
			if !gotPort {
				if p, err := strconv.Atoi(val); err == nil {
					result.Port = p
				}
				gotPort = true
			}
		}
	}

	return result
}

// HostKeyTypeFromFilename extracts the key type from an SSH host key
// filename like "ssh_host_ed25519_key" -> "ed25519".
func HostKeyTypeFromFilename(name string) string {
	name = strings.TrimPrefix(name, "ssh_host_")
	name = strings.TrimSuffix(name, "_key")
	name = strings.TrimSuffix(name, "_key.pub")
	return name
}
