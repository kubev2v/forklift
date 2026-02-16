package hyperv

import (
	core "k8s.io/api/core/v1"
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
