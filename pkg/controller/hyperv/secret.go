package hyperv

import (
	"fmt"
	"strconv"

	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
	core "k8s.io/api/core/v1"
)

const (
	SettingWinRMPort      = "winrmPort"
	SettingSMBMaxChannels = "smbMaxChannels"
	DefaultSMBMaxChannels = 0
	MaxSMBChannels        = 16
)

// Secret field names for HyperV provider.
// Required fields:
//   - username: Hyper-V host username (e.g., "Administrator")
//   - password: Hyper-V host password
//   - smbUrl: SMB share URL (e.g., "//192.168.1.100/VMShare")
//
// Optional fields:
//   - smbUser: SMB username (defaults to Hyper-V username)
//   - smbPassword: SMB password (defaults to Hyper-V password)
const (
	SecretFieldUsername    = "username"
	SecretFieldPassword    = "password"
	SecretFieldSMBUrl      = "smbUrl"
	SecretFieldSMBUser     = "smbUser"
	SecretFieldSMBPassword = "smbPassword"
)

// Pod-internal constants (not user-configurable)
const (
	// SMBMountPath is the local mount point where SMB is mounted in the pod.
	SMBMountPath = "/hyperv"
	// StorageIDDefault is the ID of the single storage record.
	StorageIDDefault = "storage-0"
)

// HyperVCredentials returns the HyperV/WinRM credentials from the secret.
func HyperVCredentials(secret *core.Secret) (username, password string) {
	username = string(secret.Data[SecretFieldUsername])
	password = string(secret.Data[SecretFieldPassword])
	return
}

// SMBCredentials returns the SMB credentials from the secret.
// Falls back to HyperV credentials if dedicated SMB credentials are not set.
func SMBCredentials(secret *core.Secret) (username, password string) {
	username = string(secret.Data[SecretFieldSMBUser])
	password = string(secret.Data[SecretFieldSMBPassword])
	if username == "" {
		username = string(secret.Data[SecretFieldUsername])
	}
	if password == "" {
		password = string(secret.Data[SecretFieldPassword])
	}
	return
}

// SMBUrl returns the SMB share URL from the secret.
func SMBUrl(secret *core.Secret) string {
	return string(secret.Data[SecretFieldSMBUrl])
}

// WinRMPort returns the WinRM port from provider settings, falling back to the default (5986).
func WinRMPort(settings map[string]string) int {
	if s, ok := settings[SettingWinRMPort]; ok && s != "" {
		if port, err := strconv.Atoi(s); err == nil && port > 0 {
			return port
		}
	}
	return driver.WinRMPortHTTPS
}

// SMBMaxChannels returns the configured SMB multichannel count from provider
// settings (0 = disabled, 1-16 = enabled with that many channels).
func SMBMaxChannels(settings map[string]string) int {
	s, ok := settings[SettingSMBMaxChannels]
	if !ok || s == "" {
		return DefaultSMBMaxChannels
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return DefaultSMBMaxChannels
	}
	if n > MaxSMBChannels {
		return MaxSMBChannels
	}
	return n
}

// SMBMountOptions returns the CIFS mount options for SMB PVs based on
// provider settings. When smbMaxChannels > 0, multichannel is enabled.
func SMBMountOptions(settings map[string]string) []string {
	channels := SMBMaxChannels(settings)
	if channels <= 0 {
		return nil
	}
	return []string{fmt.Sprintf("max_channels=%d", channels)}
}
