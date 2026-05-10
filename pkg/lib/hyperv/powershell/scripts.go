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

// DiffDiskPath returns the conventional Windows path for a VM's differencing
// disk inside the iSCSI staging directory.
// Format: C:\iscsi-targets\<targetName>-disk<index>.vhdx
func DiffDiskPath(targetName string, diskIndex int) string {
	return fmt.Sprintf(`%s\%s-disk%d.vhdx`, IscsiTargetDir, targetName, diskIndex)
}

// DiffDiskPattern returns a wildcard pattern matching all differencing disks
// for a VM inside the iSCSI staging directory.
// Format: C:\iscsi-targets\<targetName>-*
func DiffDiskPattern(targetName string) string {
	return fmt.Sprintf(`%s\%s-*`, IscsiTargetDir, targetName)
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

	// GetSMBSharePath returns the Windows path for an SMB share
	// Parameters: shareName
	GetSMBSharePath = `(Get-SmbShare -Name '%s').Path`

	// GetStorageCapacity returns the capacity and free space for a volume containing a path.
	// Returns JSON with Size (total capacity) and SizeRemaining (free space) in bytes.
	// Parameters: windowsPath
	GetStorageCapacity = `Get-Volume -FilePath '%s' | Select-Object Size, SizeRemaining | ConvertTo-Json -Compress`
)

const (
	// GetVMNICs returns all network adapters attached to a VM
	// Parameters: vmName
	GetVMNICs = `Get-VMNetworkAdapter -VMName '%s' | Select-Object Name, MacAddress, SwitchName | ConvertTo-Json`
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

const (
	// CheckIscsiTargetFeature checks whether the iSCSI Target Server Windows feature is installed.
	// Returns JSON: {"Installed": true/false}
	CheckIscsiTargetFeature = `$f=Get-WindowsFeature FS-iSCSITarget-Server -ErrorAction SilentlyContinue
if($f){[PSCustomObject]@{Installed=[bool]$f.Installed}|ConvertTo-Json -Compress}else{[PSCustomObject]@{Installed=$false}|ConvertTo-Json -Compress}`

	// CheckIscsiFirewallPort checks whether TCP 3260 has an active listener via Get-NetTCPConnection.
	// Test-NetConnection against localhost is unreliable on some Windows versions.
	// Returns JSON: {"Open": true/false}
	CheckIscsiFirewallPort = `$l=Get-NetTCPConnection -LocalPort 3260 -State Listen -ErrorAction SilentlyContinue
if($l){[PSCustomObject]@{Open=$true}|ConvertTo-Json -Compress}else{[PSCustomObject]@{Open=$false}|ConvertTo-Json -Compress}`
)

const (
	// CreateIscsiTarget creates an iSCSI Server Target with IQN-based initiator ACL.
	// If a target with the same name exists, the initiator IQN is appended to the
	// existing ACL rather than replacing it, preserving any other legitimate entries.
	// Parameters: targetName, initiatorIQN
	CreateIscsiTarget = `$name='%s'
$iqn='%s'
$existing=Get-IscsiServerTarget -TargetName $name -ErrorAction SilentlyContinue
if($existing){$ids=@($existing.InitiatorIds);if($ids -notcontains "IQN:$iqn"){$ids+="IQN:$iqn"};Set-IscsiServerTarget -TargetName $name -InitiatorIds $ids -ErrorAction Stop;$updated=Get-IscsiServerTarget -TargetName $name;[PSCustomObject]@{TargetIqn=[string]$updated.TargetIqn;Created=$false;InitiatorIds=($updated.InitiatorIds -join ',')}|ConvertTo-Json -Compress;return}
$t=New-IscsiServerTarget -TargetName $name -InitiatorIds @("IQN:$iqn") -ErrorAction Stop
[PSCustomObject]@{TargetIqn=[string]$t.TargetIqn;Created=$true;InitiatorIds=($t.InitiatorIds -join ',')}|ConvertTo-Json -Compress`

	// RemoveIscsiTarget removes an iSCSI Server Target and all its virtual disk mappings.
	// Idempotent: succeeds even if the target does not exist.
	// Parameters: targetName
	RemoveIscsiTarget = `$name='%s'
$t=Get-IscsiServerTarget -TargetName $name -ErrorAction SilentlyContinue
if(-not $t){return}
$mappings=@($t.LunMappings)
$unmapErrors=@()
foreach($m in $mappings){try{Remove-IscsiVirtualDiskTargetMapping -TargetName $name -Path $m.Path -ErrorAction Stop}catch{$unmapErrors+=$_.Exception.Message}}
foreach($m in $mappings){try{Remove-IscsiVirtualDisk -Path $m.Path -ErrorAction Stop}catch{}}
$retries=3;$removed=$false
for($i=0;$i -lt $retries;$i++){Remove-IscsiServerTarget -TargetName $name -ErrorAction SilentlyContinue;$check=Get-IscsiServerTarget -TargetName $name -ErrorAction SilentlyContinue;if(-not $check){$removed=$true;break};Start-Sleep -Seconds 2}
if(-not $removed){throw "Failed to remove iSCSI target '$name' after $retries attempts. Unmap errors: $($unmapErrors -join '; ')"}`

	// GetIscsiTarget retrieves information about an existing iSCSI Server Target.
	// Returns JSON with TargetIqn and Status, or empty string if not found.
	// Parameters: targetName
	GetIscsiTarget = `$t=Get-IscsiServerTarget -TargetName '%s' -ErrorAction SilentlyContinue
if($t){[PSCustomObject]@{TargetIqn=[string]$t.TargetIqn;Status=$t.Status.ToString();LunCount=@($t.LunMappings).Count}|ConvertTo-Json -Compress}`
)

// iSCSI virtual disk (LUN) management — differencing disks and target mappings.
//
// The workflow is: EnsureIscsiTargetDir → CreateIscsiVirtualDisk (per VHDX) →
// AddIscsiVirtualDiskTargetMapping (per disk) → copy → RemoveIscsiVirtualDiskTargetMapping →
// RemoveIscsiVirtualDisk → (eventually) RemoveIscsiTarget.
const (
	// IscsiTargetDir is the Windows directory where differencing disks are stored.
	// Created once per host, cleaned up per-VM after copy completes.
	IscsiTargetDir = `C:\iscsi-targets`

	// EnsureIscsiTargetDir creates the iSCSI staging directory if it doesn't exist.
	// Idempotent — no error if the directory already exists.
	EnsureIscsiTargetDir = `$d='%s';if(-not (Test-Path $d)){New-Item -Path $d -ItemType Directory -Force | Out-Null}`

	// CreateIscsiVirtualDisk creates a differencing disk referencing an existing VHDX.
	// The differencing disk is a thin metadata file (<1 MB) that references the original
	// VHDX. The iSCSI Target Server serves the logical content (raw guest disk), not the
	// VHDX container format.
	// Returns JSON: {"DevicePath": "C:\\iscsi-targets\\forklift-<vmId>-disk0.vhdx"}
	// Parameters: diffDiskPath, parentVhdxPath
	CreateIscsiVirtualDisk = `$diffPath='%s'
$parentPath='%s'
$existing=Get-IscsiVirtualDisk -Path $diffPath -ErrorAction SilentlyContinue
if($existing){
  $vhd=Get-VHD -Path $diffPath -ErrorAction SilentlyContinue
  if($vhd -and $vhd.ParentPath -ne $parentPath){
    Remove-IscsiVirtualDisk -Path $diffPath -ErrorAction SilentlyContinue
    if(Test-Path $diffPath){Remove-Item -Path $diffPath -Force}
  }else{
    [PSCustomObject]@{DevicePath=$existing.Path}|ConvertTo-Json -Compress;return
  }
}
$vd=New-IscsiVirtualDisk -Path $diffPath -ParentPath $parentPath -ErrorAction Stop
[PSCustomObject]@{DevicePath=$vd.Path}|ConvertTo-Json -Compress`

	// AddIscsiVirtualDiskTargetMapping maps a virtual disk to an iSCSI target at a specific LUN.
	// The LUN number determines which /dev/disk/by-path/ip-*-lun-N device the initiator sees.
	// Idempotent: if the mapping already exists, the command succeeds silently.
	// Parameters: targetName, diffDiskPath, lunID
	AddIscsiVirtualDiskTargetMapping = `$target='%s'
$diskPath='%s'
$lun=%s
Add-IscsiVirtualDiskTargetMapping -TargetName $target -Path $diskPath -Lun $lun -ErrorAction Stop`

	// RemoveIscsiVirtualDiskTargetMapping removes a single disk mapping from a target.
	// Idempotent: succeeds even if the mapping does not exist.
	// Parameters: targetName, diffDiskPath
	RemoveIscsiVirtualDiskTargetMapping = `Remove-IscsiVirtualDiskTargetMapping -TargetName '%s' -Path '%s' -ErrorAction SilentlyContinue`

	// RemoveIscsiVirtualDisk removes a single iSCSI virtual disk and deletes the
	// differencing disk file from the filesystem.
	// Idempotent: succeeds even if the disk does not exist.
	// Parameters: diffDiskPath
	RemoveIscsiVirtualDisk = `$p='%s'
Remove-IscsiVirtualDisk -Path $p -ErrorAction SilentlyContinue
if(Test-Path $p){Remove-Item -Path $p -Force -ErrorAction SilentlyContinue}`

	// CleanupIscsiDiffDisks removes all differencing disk mappings, virtual disks, and files
	// for a specific VM from an iSCSI target. Called after copy completes (or on failure).
	// The target itself is NOT removed — it may be reused on retry.
	// Parameters: targetName, vmFilePattern (e.g. "C:\iscsi-targets\forklift-<vmId>-*")
	CleanupIscsiDiffDisks = `$target='%s'
$pattern='%s'
$t=Get-IscsiServerTarget -TargetName $target -ErrorAction SilentlyContinue
if($t){$mappings=@($t.LunMappings);foreach($m in $mappings){if($m.Path -like $pattern){Remove-IscsiVirtualDiskTargetMapping -TargetName $target -Path $m.Path -ErrorAction SilentlyContinue;Remove-IscsiVirtualDisk -Path $m.Path -ErrorAction SilentlyContinue}}}
Get-ChildItem -Path $pattern -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue`

	// GetIscsiVirtualDiskTargetMappings lists all LUN mappings for a target.
	// Returns JSON array with Path and Lun for each mapping.
	// Parameters: targetName
	GetIscsiVirtualDiskTargetMappings = `$t=Get-IscsiServerTarget -TargetName '%s' -ErrorAction SilentlyContinue
if($t){@($t.LunMappings) | ForEach-Object{[PSCustomObject]@{Path=$_.Path;Lun=$_.Lun}} | ConvertTo-Json -Compress}`
)

// Disk validation via WinRM.
const (
	// TestPath checks whether a file exists on the Hyper-V host.
	// Returns "True" or "False".
	// Parameters: windowsPath
	TestPath = `Test-Path -Path '%s' -PathType Leaf`

	// TestPaths checks multiple file paths in a single WinRM call.
	// Returns JSON: {"Missing": ["path1", "path3"]} for paths that don't exist.
	// Parameter: comma-separated quoted strings, e.g. "'C:\\path1','C:\\path2'"
	TestPaths = `$paths=@(%s)
$missing=@()
foreach($p in $paths){if(-not (Test-Path -Path $p -PathType Leaf)){$missing+=$p}}
[PSCustomObject]@{Missing=$missing}|ConvertTo-Json -Compress`
)
