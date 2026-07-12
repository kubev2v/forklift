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

// RunOnNode wraps cmd to run on a remote node via Invoke-Command -Credential.
// If computerName is empty, returns cmd unchanged (runs on the connected host).
func RunOnNode(cmd, computerName, password, username string) string {
	if computerName == "" {
		return cmd
	}
	escPass := strings.ReplaceAll(password, "'", "''")
	escUser := strings.ReplaceAll(username, "'", "''")
	escNode := strings.ReplaceAll(computerName, "'", "''")
	return fmt.Sprintf(
		"$pw = ConvertTo-SecureString '%s' -AsPlainText -Force; "+
			"$cred = New-Object System.Management.Automation.PSCredential('%s', $pw); "+
			"Invoke-Command -ComputerName '%s' -Credential $cred -ScriptBlock { %s }",
		escPass, escUser, escNode, cmd,
	)
}

const (
	// TestConnection verifies WinRM connectivity
	TestConnection = `echo ok`
)

const (
	// ListAllVMs returns all VMs with basic properties
	ListAllVMs = `Get-VM | Select-Object Id, Name, State, ProcessorCount, MemoryStartup, Generation | ConvertTo-Json`

	// ListClusterVMs collects VMs from all cluster nodes using Invoke-Command
	// with explicit credentials to avoid WinRM double-hop issues.
	// Remote State enums must be cast to [int] to avoid complex JSON objects.
	// Parameters: password, username
	ListClusterVMs = `$pw = ConvertTo-SecureString '%s' -AsPlainText -Force; $cred = New-Object System.Management.Automation.PSCredential('%s', $pw); $localName = $env:COMPUTERNAME; $allVMs = @(); $allVMs += Get-VM | Select-Object Id, Name, @{N='State';E={[int]$_.State}}, ProcessorCount, MemoryStartup, Generation, @{N='ComputerName';E={$localName}}; Get-ClusterNode | Where-Object { $_.Name -ne $localName -and $_.State -eq 0 } | ForEach-Object { $node = $_.Name; try { $remote = Invoke-Command -ComputerName $node -Credential $cred -ScriptBlock { Get-VM | Select-Object Id, Name, @{N='State';E={[int]$_.State}}, ProcessorCount, MemoryStartup, Generation } -ErrorAction Stop; $remote | ForEach-Object { $_ | Add-Member -NotePropertyName ComputerName -NotePropertyValue $node -Force; $allVMs += $_ } } catch { Write-Warning "Node ${node}: $($_.Exception.Message)" } }; $allVMs | ConvertTo-Json`

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
	// GetVMNICs returns all network adapters attached to a VM including VLAN configuration.
	// Only Access-mode VLANs are captured, Trunk-mode adapters (NativeVlanId) report 0.
	// Parameters: vmName
	GetVMNICs = `Get-VMNetworkAdapter -VMName '%s' | ForEach-Object { $v = ($_ | Get-VMNetworkAdapterVlan); $_ | Select-Object Name, MacAddress, SwitchName, @{N='VlanId';E={if($v.AccessVlanId){$v.AccessVlanId}else{0}}} } | ConvertTo-Json`
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
	// Returns the OS name string (e.g., "Microsoft Windows Server 2019 Standard").
	// If OSName lacks a version number, appends OSMajorVersion from KVP
	// (e.g., "Red Hat Enterprise Linux" + "9" -> "Red Hat Enterprise Linux 9").
	// Returns empty string if VM not found or Integration Services unavailable.
	// Parameters: vmName
	GetGuestOS = `$vm=Get-CimInstance -Namespace root\virtualization\v2 -ClassName Msvm_ComputerSystem -Filter "ElementName='%s'" -ErrorAction SilentlyContinue
if(-not $vm){return}
$kvp=Get-CimAssociatedInstance -InputObject $vm -ResultClassName Msvm_KvpExchangeComponent -ErrorAction SilentlyContinue
if(-not $kvp -or -not $kvp.GuestIntrinsicExchangeItems){return}
$osName='';$osMajor=''
foreach($item in $kvp.GuestIntrinsicExchangeItems){$xml=[xml]$item;$n=$xml.INSTANCE.PROPERTY|Where-Object{$_.NAME -eq 'Name'}|Select-Object -ExpandProperty VALUE;$v=$xml.INSTANCE.PROPERTY|Where-Object{$_.NAME -eq 'Data'}|Select-Object -ExpandProperty VALUE;if($n -eq 'OSName'){$osName=$v}elseif($n -eq 'OSMajorVersion'){$osMajor=$v}}
if($osName -and $osMajor -and $osName -notmatch '\d'){$osName="$osName $osMajor"}
$osName`
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

// Failover Clustering scripts.
// Cluster-level scripts run on the CNO (Cluster Name Object) entry node.
// Per-node scripts run on direct WinRM connections to individual cluster nodes.
const (
	// GetCluster returns the Failover Cluster identity (name and domain).
	// Run on the CNO / entry node connection.
	GetCluster = `Get-Cluster | Select-Object Name, Domain | ConvertTo-Json`

	// GetClusterNodes returns all nodes in the Failover Cluster with their state.
	// Node State values: Up, Down, Paused, Joining.
	// Run on the CNO / entry node connection.
	GetClusterNodes = `Get-ClusterNode | Select-Object Name, @{N='State';E={[int]$_.State}}, Id | ConvertTo-Json`

	// GetClusterVMGroups returns all VM roles in the cluster with their owner node.
	// GroupType 'VirtualMachine' filters to only Hyper-V VM cluster roles.
	// OwnerNode indicates which node currently runs the VM (real-time).
	// Run on the CNO / entry node connection.
	GetClusterVMGroups = `Get-ClusterGroup | Where-Object {$_.GroupType -eq 'VirtualMachine'} | Select-Object Name, @{N='OwnerNode';E={$_.OwnerNode.Name}}, @{N='State';E={[int]$_.State}}, Id | ConvertTo-Json`
)

const (
	// GetComputerInfo returns hardware information for a cluster node.
	// CsNumberOfProcessors = physical CPU sockets,
	// CsNumberOfLogicalProcessors = total logical CPUs (cores x threads),
	// OsTotalVisibleMemorySize = total RAM in KB,
	// CsDNSHostName = node hostname, CsDomain = AD domain.
	// Run on a direct per-node WinRM connection.
	GetComputerInfo = `Get-ComputerInfo | Select-Object CsDNSHostName, CsDomain, CsNumberOfProcessors, CsNumberOfLogicalProcessors, OsTotalVisibleMemorySize | ConvertTo-Json`
)

// Batch VM detail scripts — split into two to fit WinRM's command-line limit.
// Each returns JSON keyed by VM name.
const (
	// BatchGetVMHardware collects security info, checkpoint status, disk
	// topology+capacity+RCT, and NIC info for all VMs on the host.
	// Disk entries include controller type/number/location so the caller
	// can build full Disk objects without per-VM WinRM round-trips.
	BatchGetVMHardware = `$r=@{};foreach($vm in(Get-VM)){$n=$vm.Name;$e=@{};if($vm.Generation-eq 2){$s=Get-VMSecurity -VMName $n -EA 0;$f=Get-VMFirmware -VMName $n -EA 0;$t=$false;$b=$false;if($s){$t=$s.TpmEnabled};if($f-and$f.SecureBoot-eq'On'){$b=$true};$e['Security']=@{TpmEnabled=$t;SecureBoot=$b}}else{$e['Security']=@{TpmEnabled=$false;SecureBoot=$false}};$e['HasCheckpoint']=[bool](Get-VMSnapshot -VMName $n -EA 0);$dd=@();foreach($d in(Get-VMHardDiskDrive -VMName $n -EA 0)){if(-not$d.Path){continue};$v=Get-VHD -Path $d.Path -EA 0;$c=0;$rc=$false;if($v){$c=$v.Size;if($v.RctId){$rc=$true}};$dd+=@{Path=$d.Path;Capacity=$c;RCTEnabled=$rc;CT=[int]$d.ControllerType;CN=$d.ControllerNumber;CL=$d.ControllerLocation}};$e['Disks']=$dd;$nn=@();foreach($a in(Get-VMNetworkAdapter -VMName $n -EA 0)){$vl=0;$vi=$a|Get-VMNetworkAdapterVlan -EA 0;if($vi-and$vi.AccessVlanId){$vl=$vi.AccessVlanId};$nn+=@{Name=$a.Name;MAC=$a.MacAddress;Switch=$a.SwitchName;Vlan=$vl}};$e['NICs']=$nn;$r[$n]=$e};$r|ConvertTo-Json -Depth 4 -Compress`

	// BatchGetVMGuest collects guest OS and guest network config for running VMs.
	BatchGetVMGuest = `$r=@{};foreach($vm in(Get-VM|?{$_.State-eq'Running'})){$n=$vm.Name;$e=@{};$ci=Get-CimInstance -Namespace root\virtualization\v2 -ClassName Msvm_ComputerSystem -Filter "ElementName='$n'" -EA 0;if($ci){$kv=Get-CimAssociatedInstance -InputObject $ci -ResultClassName Msvm_KvpExchangeComponent -EA 0;if($kv-and$kv.GuestIntrinsicExchangeItems){$os='';$om='';foreach($i in $kv.GuestIntrinsicExchangeItems){$x=[xml]$i;$pn=$x.INSTANCE.PROPERTY|?{$_.NAME-eq'Name'}|Select -Exp VALUE;$pv=$x.INSTANCE.PROPERTY|?{$_.NAME-eq'Data'}|Select -Exp VALUE;if($pn-eq'OSName'){$os=$pv}elseif($pn-eq'OSMajorVersion'){$om=$pv}};if($os-and$om-and$os-notmatch'\d'){$os="$os $om"};$e['GuestOS']=$os};$vs=Get-CimAssociatedInstance -InputObject $ci -ResultClassName Msvm_VirtualSystemSettingData|?{$_.VirtualSystemType-eq'Microsoft:Hyper-V:System:Realized'};if($vs){$ps=Get-CimAssociatedInstance -InputObject $vs -ResultClassName Msvm_SyntheticEthernetPortSettingData;$nc=@();foreach($p in $ps){$gc=Get-CimAssociatedInstance -InputObject $p -ResultClassName Msvm_GuestNetworkAdapterConfiguration;if($gc){$nc+=[PSCustomObject]@{MAC=$p.Address;IPs=$gc.IPAddresses;Subnets=$gc.Subnets;DHCP=$gc.DHCPEnabled;GW=$gc.DefaultGateways;DNS=$gc.DNSServers}}};if($nc.Count-gt 0){$e['GuestNetworks']=$nc}}};if($e.Count-gt 0){$r[$n]=$e}};$r|ConvertTo-Json -Depth 4 -Compress`
)
