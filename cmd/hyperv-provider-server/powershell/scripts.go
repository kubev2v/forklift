// Package powershell contains PowerShell script templates for Hyper-V operations.
// All scripts are defined as constants for easy maintenance and review.
package powershell

import (
	"fmt"
	"strings"
)

func BuildCommand(template string, args ...string) string {
	sanitized := make([]any, len(args))
	for i, arg := range args {
		// Escape single quotes by doubling them
		sanitized[i] = strings.ReplaceAll(arg, "'", "''")
	}
	return fmt.Sprintf(template, sanitized...)
}

const (
	// TestConnection verifies WinRM connectivity
	TestConnection = `echo ok`
)

const (
	// ListAllVMs returns all VMs with basic properties
	ListAllVMs = `Get-VM | Select-Object Id, Name, State, ProcessorCount, MemoryStartup, Generation | ConvertTo-Json`

	// GetVMByName returns a single VM by name
	// Parameters: vmName
	GetVMByName = `Get-VM -Name '%s' | Select-Object Id, Name, State, ProcessorCount, MemoryStartup, Generation | ConvertTo-Json`

	// GetVMByID returns a single VM by ID
	// Parameters: vmId
	GetVMByID = `Get-VM -Id '%s' | Select-Object Id, Name, State, ProcessorCount, MemoryStartup, Generation | ConvertTo-Json`

	// StopVM forcefully stops a VM
	// Parameters: vmName
	StopVM = `Stop-VM -Name '%s' -Force -Confirm:$false`
)

const (
	// ListAllSwitches returns all virtual switches
	ListAllSwitches = `Get-VMSwitch | Select-Object Id, Name, SwitchType | ConvertTo-Json`
)

const (
	// GetVMDisks returns all hard disk drives attached to a VM
	// Parameters: vmName
	GetVMDisks = `Get-VMHardDiskDrive -VMName '%s' | Select-Object Path, ControllerType, ControllerNumber, ControllerLocation | ConvertTo-Json`

	// GetDiskCapacity returns the size of a VHD/VHDX file in bytes
	// Parameters: windowsPath
	GetDiskCapacity = `(Get-VHD -Path '%s').Size`
)

const (
	// GetVMNICs returns all network adapters attached to a VM
	// Parameters: vmName
	GetVMNICs = `Get-VMNetworkAdapter -VMName '%s' | Select-Object Name, MacAddress, SwitchName | ConvertTo-Json`
)

const (
	// GetSMBSharePath returns the Windows path for an SMB share
	// Parameters: shareName
	GetSMBSharePath = `(Get-SmbShare -Name '%s').Path`

	// GetStorageCapacity returns the capacity and free space for a volume containing a path.
	// Returns JSON with Size (total capacity) and SizeRemaining (free space) in bytes.
	// Parameters: windowsPath
	GetStorageCapacity = `Get-Volume -FilePath '%s' | Select-Object Size, SizeRemaining | ConvertTo-Json -Compress`
)

const (
	// GetGuestNetworkConfig retrieves IP configuration from a running VM via KVP Exchange.
	// Requires: VM running, Integration Services installed, Data Exchange enabled.
	// Returns JSON with MAC, IPs, Subnets, DHCP status, Gateways, DNS servers.
	// Parameters: vmName
	GetGuestNetworkConfig = `$vm=Get-CimInstance -Namespace root\virtualization\v2 -ClassName Msvm_ComputerSystem -Filter "ElementName='%s'"
if(-not $vm){'no_vm';return}
$vs=Get-CimAssociatedInstance -InputObject $vm -ResultClassName Msvm_VirtualSystemSettingData|Where-Object{$_.VirtualSystemType -eq 'Microsoft:Hyper-V:System:Realized'}
$ports=Get-CimAssociatedInstance -InputObject $vs -ResultClassName Msvm_SyntheticEthernetPortSettingData
$r=@()
foreach($p in $ports){$gc=Get-CimAssociatedInstance -InputObject $p -ResultClassName Msvm_GuestNetworkAdapterConfiguration;if($gc){$r+=[PSCustomObject]@{MAC=$p.Address;IPs=$gc.IPAddresses;Subnets=$gc.Subnets;DHCP=$gc.DHCPEnabled;GW=$gc.DefaultGateways;DNS=$gc.DNSServers}}}
if($r.Count -gt 0){$r|ConvertTo-Json -Compress}else{'no_gc'}`
)

const (
	// GetGuestOS retrieves guest operating system name via KVP Exchange.
	// Requires: VM running, Integration Services installed, Data Exchange enabled.
	// Returns the OS name string (e.g., "Microsoft Windows Server 2019 Standard"),
	// or empty string if VM not found or Integration Services unavailable.
	// Parameters: vmName
	GetGuestOS = `$vm=Get-CimInstance -Namespace root\virtualization\v2 -ClassName Msvm_ComputerSystem -Filter "ElementName='%s'" -ErrorAction SilentlyContinue
if(-not $vm){return}
$kvp=Get-CimAssociatedInstance -InputObject $vm -ResultClassName Msvm_KvpExchangeComponent -ErrorAction SilentlyContinue
if(-not $kvp -or -not $kvp.GuestIntrinsicExchangeItems){return}
foreach($item in $kvp.GuestIntrinsicExchangeItems){$xml=[xml]$item;$name=$xml.INSTANCE.PROPERTY|Where-Object{$_.NAME -eq 'Name'}|Select-Object -ExpandProperty VALUE;$value=$xml.INSTANCE.PROPERTY|Where-Object{$_.NAME -eq 'Data'}|Select-Object -ExpandProperty VALUE;if($name -eq 'OSName'){$value;return}}`
)

const (
	// GetVMSecurityInfo retrieves TPM and security settings for a VM.
	// Only Gen2 VMs support TPM. Returns JSON with TpmEnabled and SecureBoot.
	// Parameters: vmName (used for Get-VM, Get-VMSecurity, Get-VMFirmware)
	GetVMSecurityInfo = `$vm=Get-VM -Name '%s' -ErrorAction SilentlyContinue
if(-not $vm){return '{}'}
if($vm.Generation -ne 2){return '{"TpmEnabled":false,"SecureBoot":false}'}
$sec=Get-VMSecurity -VMName '%s' -ErrorAction SilentlyContinue
$fw=Get-VMFirmware -VMName '%s' -ErrorAction SilentlyContinue
$tpm=$false;$sb=$false
if($sec){$tpm=$sec.TpmEnabled}
if($fw -and $fw.SecureBoot -eq 'On'){$sb=$true}
[PSCustomObject]@{TpmEnabled=$tpm;SecureBoot=$sb}|ConvertTo-Json -Compress`
)

const (
	// GetVMHasCheckpoint checks if VM has existing checkpoints/snapshots.
	// Returns "true" if snapshots exist, "false" otherwise.
	// Parameters: vmName
	GetVMHasCheckpoint = `$snaps=Get-VMSnapshot -VMName '%s' -ErrorAction SilentlyContinue
if($snaps){'true'}else{'false'}`
)

const (
	// GetDiskRCTEnabled checks if Resilient Change Tracking is enabled for a disk.
	// RCT is required for warm migration.
	// Returns "true" if RCT is enabled (RctId is set), "false" otherwise.
	// Parameters: windowsPath
	GetDiskRCTEnabled = `$vhd=Get-VHD -Path '%s' -ErrorAction SilentlyContinue
if($vhd -and $vhd.RctId){'true'}else{'false'}`
)
