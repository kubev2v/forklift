// Package powershell holds Hyper-V PowerShell script templates.
package powershell

import (
	"fmt"
	"strings"
)

func BuildCommand(template string, args ...string) string {
	sanitized := make([]any, len(args))
	for i, arg := range args {
		sanitized[i] = strings.ReplaceAll(arg, "'", "''") // PS single-quote escape
	}
	return fmt.Sprintf(template, sanitized...)
}

// RunOnNode wraps cmd in Invoke-Command -Credential. Empty computerName = local.
// Bare usernames get a ".\ " prefix for PSCredential.
func RunOnNode(cmd, computerName, password, username string) string {
	if computerName == "" {
		return cmd
	}
	credUser := username
	if !strings.Contains(credUser, `\`) && !strings.Contains(credUser, "@") {
		credUser = `.\` + credUser
	}
	escPass := strings.ReplaceAll(password, "'", "''")
	escUser := strings.ReplaceAll(credUser, "'", "''")
	escNode := strings.ReplaceAll(computerName, "'", "''")
	return fmt.Sprintf(
		"$pw = ConvertTo-SecureString '%s' -AsPlainText -Force; "+
			"$cred = New-Object System.Management.Automation.PSCredential('%s', $pw); "+
			"Invoke-Command -ComputerName '%s' -Credential $cred -ScriptBlock { %s }",
		escPass, escUser, escNode, cmd,
	)
}

const (
	TestConnection = `echo ok`
)

const (
	ListAllVMs = `Get-VM | Select-Object Id, Name, State, ProcessorCount, MemoryStartup, Generation | ConvertTo-Json`

	// ListClusterVMs: all cluster nodes via Invoke-Command (avoids WinRM double-hop).
	// State cast to [int]. Parameters: password, username
	ListClusterVMs = `$pw = ConvertTo-SecureString '%s' -AsPlainText -Force; $cred = New-Object System.Management.Automation.PSCredential('%s', $pw); $localName = $env:COMPUTERNAME; $allVMs = @(); $allVMs += Get-VM | Select-Object Id, Name, @{N='State';E={[int]$_.State}}, ProcessorCount, MemoryStartup, Generation, @{N='ComputerName';E={$localName}}; Get-ClusterNode | Where-Object { $_.Name -ne $localName -and $_.State -eq 0 } | ForEach-Object { $node = $_.Name; try { $remote = Invoke-Command -ComputerName $node -Credential $cred -ScriptBlock { Get-VM | Select-Object Id, Name, @{N='State';E={[int]$_.State}}, ProcessorCount, MemoryStartup, Generation } -ErrorAction Stop; $remote | ForEach-Object { $_ | Add-Member -NotePropertyName ComputerName -NotePropertyValue $node -Force; $allVMs += $_ } } catch { Write-Warning "Node ${node}: $($_.Exception.Message)" } }; $allVMs | ConvertTo-Json`

	// Parameters: vmName
	GetVMByName = `Get-VM -Name '%s' | Select-Object Id, Name, State, ProcessorCount, MemoryStartup, Generation | ConvertTo-Json`

	// Parameters: vmId
	GetVMByID = `Get-VM -Id '%s' | Select-Object Id, Name, State, ProcessorCount, MemoryStartup, Generation | ConvertTo-Json`

	// Parameters: vmName
	GetVMState = `Get-VM -Name '%s' | Select-Object -ExpandProperty State`

	// Parameters: vmName
	StopVM = `Stop-VM -Name '%s' -Force -Confirm:$false`
)

const (
	ListAllSwitches = `Get-VMSwitch | Select-Object Id, Name, SwitchType | ConvertTo-Json`
)

const (
	// Parameters: vmName
	GetVMDisks = `Get-VMHardDiskDrive -VMName '%s' | Select-Object Path, ControllerType, ControllerNumber, ControllerLocation | ConvertTo-Json`

	// Parameters: windowsPath
	GetDiskCapacity = `(Get-VHD -Path '%s').Size`
)

const (
	// GetVMNICs: Access VLAN only (Trunk → 0). Parameters: vmName
	GetVMNICs = `Get-VMNetworkAdapter -VMName '%s' | ForEach-Object { $v = ($_ | Get-VMNetworkAdapterVlan); $_ | Select-Object Name, MacAddress, SwitchName, @{N='VlanId';E={if($v.AccessVlanId){$v.AccessVlanId}else{0}}} } | ConvertTo-Json`
)

const (
	// Parameters: shareName
	GetSMBSharePath = `(Get-SmbShare -Name '%s').Path`

	// Size / SizeRemaining in bytes. Parameters: windowsPath
	GetStorageCapacity = `Get-Volume -FilePath '%s' | Select-Object Size, SizeRemaining | ConvertTo-Json -Compress`
)

const (
	// GetGuestNetworkConfig via KVP (VM running + IS). Parameters: vmName
	GetGuestNetworkConfig = `$vm=Get-CimInstance -Namespace root\virtualization\v2 -ClassName Msvm_ComputerSystem -Filter "ElementName='%s'"
if(-not $vm){'no_vm';return}
$vs=Get-CimAssociatedInstance -InputObject $vm -ResultClassName Msvm_VirtualSystemSettingData|Where-Object{$_.VirtualSystemType -eq 'Microsoft:Hyper-V:System:Realized'}
$ports=Get-CimAssociatedInstance -InputObject $vs -ResultClassName Msvm_SyntheticEthernetPortSettingData
$r=@()
foreach($p in $ports){$gc=Get-CimAssociatedInstance -InputObject $p -ResultClassName Msvm_GuestNetworkAdapterConfiguration;if($gc){$r+=[PSCustomObject]@{MAC=$p.Address;IPs=$gc.IPAddresses;Subnets=$gc.Subnets;DHCP=$gc.DHCPEnabled;GW=$gc.DefaultGateways;DNS=$gc.DNSServers}}}
if($r.Count -gt 0){$r|ConvertTo-Json -Compress}else{'no_gc'}`
)

const (
	// GetGuestOS via KVP, appends OSMajorVersion when OSName has no digits. Parameters: vmName
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
	// GetVMSecurityInfo: TPM/SecureBoot JSON (Gen2 only). Parameters: vmName ×3
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
	// Parameters: vmName
	GetVMHasCheckpoint = `$snaps=Get-VMSnapshot -VMName '%s' -ErrorAction SilentlyContinue
if($snaps){'true'}else{'false'}`
)

const (
	// Parameters: windowsPath
	GetDiskRCTEnabled = `$vhd=Get-VHD -Path '%s' -ErrorAction SilentlyContinue
if($vhd -and $vhd.RctId){'true'}else{'false'}`
)

// Warm migration checkpoints (standard → frozen AVHDX deltas).
const (
	// Parameters: vmName, checkpointName
	CreateVMCheckpoint = `Checkpoint-VM -VMName '%s' -SnapshotName '%s' -Confirm:$false -ErrorAction Stop; 'ok'`

	// Parameters: vmName, checkpointName
	RemoveVMCheckpoint = `Remove-VMSnapshot -VMName '%s' -Name '%s' -Confirm:$false -ErrorAction SilentlyContinue; 'ok'`

	// RemoveStaleForkliftCheckpoints: delete leftover forklift-warm-*. Parameters: vmName
	RemoveStaleForkliftCheckpoints = `$removed=@();Get-VMSnapshot -VMName '%s' -EA Stop|Where-Object{$_.Name-like'forklift-warm-*'}|ForEach-Object{Remove-VMSnapshot -VMSnapshot $_ -Confirm:$false -EA Stop;$removed+=$_.Name};if($removed){$removed-join','}`

	// GetCheckpointDiskPaths: Active/Parent/Root/BaseParent/Chain JSON for warm deltas.
	// Root walk covers .avhdx and legacy .avhd. Active, Chain, and Root/BaseParent
	// Get-VHD failures throw — a truncated Chain would break mid-chain inventory
	// matching in GetSnapshotDeltas. End of chain is ParentPath empty, not a
	// failed lookup.
	// Parameters: vmName
	GetCheckpointDiskPaths = `$drives=Get-VMHardDiskDrive -VMName '%s' -EA Stop
$r=@();foreach($d in $drives){if(-not $d.Path){continue}
$root=$d.Path;$vhd=Get-VHD -Path $root -EA 0
if(-not $vhd){throw "Get-VHD failed for active disk $root"}
$parent='';if($vhd.ParentPath){$parent=$vhd.ParentPath}
$chain=@($d.Path);$cur=$parent;while($cur){$chain+=$cur;$cv=Get-VHD -Path $cur -EA 0;if(-not $cv){throw "Get-VHD failed for chain disk $cur"};if(-not $cv.ParentPath){break};$cur=$cv.ParentPath}
while($vhd -and $vhd.ParentPath -and ($root -like '*.avhdx' -or $root -like '*.avhd')){$root=$vhd.ParentPath;$vhd=Get-VHD -Path $root -EA 0;if(-not $vhd){throw "Get-VHD failed for root disk $root"}}
$bvhd=Get-VHD -Path $root -EA 0;if(-not $bvhd){throw "Get-VHD failed for root disk $root"}
$baseParent='';if($bvhd.ParentPath){$baseParent=$bvhd.ParentPath}
$r+=@{Active=$d.Path;Parent=$parent;Root=$root;BaseParent=$baseParent;Chain=$chain}}
$r|ConvertTo-Json -Compress -Depth 5`
)

// Failover Clustering (run on CNO unless noted).
const (
	GetCluster = `Get-Cluster | Select-Object Name, Domain | ConvertTo-Json`

	GetClusterNodes = `Get-ClusterNode | Select-Object Name, @{N='State';E={[int]$_.State}}, Id | ConvertTo-Json`

	// GetClusterInfo: {"Cluster":...,"Nodes":...} in one WinRM call.
	GetClusterInfo = `$c = Get-Cluster | Select-Object Name, Domain; $n = @(Get-ClusterNode | Select-Object Name, @{N='State';E={[int]$_.State}}, Id); @{Cluster=$c; Nodes=$n} | ConvertTo-Json -Depth 3`

	// GetClusterVMGroups: VirtualMachine roles + OwnerNode.
	GetClusterVMGroups = `Get-ClusterGroup | Where-Object {$_.GroupType -eq 'VirtualMachine'} | Select-Object Name, @{N='OwnerNode';E={$_.OwnerNode.Name}}, @{N='State';E={[int]$_.State}}, Id | ConvertTo-Json`
)

const (
	// GetComputerInfo via Win32_* (faster than Get-ComputerInfo), field names unchanged.
	GetComputerInfo = `$cs=Get-CimInstance Win32_ComputerSystem;$os=Get-CimInstance Win32_OperatingSystem;@{CsDNSHostName=$cs.DNSHostName;CsDomain=$cs.Domain;CsNumberOfProcessors=[int]$cs.NumberOfProcessors;CsNumberOfLogicalProcessors=[int]$cs.NumberOfLogicalProcessors;OsTotalVisibleMemorySize=[int64]$os.TotalVisibleMemorySize}|ConvertTo-Json`
)

// Batch VM detail scripts. BatchGetVMDetails is primary, Hardware/Guest are split fallbacks.
const (
	// BatchGetVMDetails: security, checkpoints, disks, NICs, guest (running) — keyed by VM name.
	BatchGetVMDetails = `$r=@{};foreach($vm in(Get-VM)){$n=$vm.Name;$e=@{};if($vm.Generation-eq 2){$s=Get-VMSecurity -VMName $n -EA 0;$f=Get-VMFirmware -VMName $n -EA 0;$t=$false;$b=$false;if($s){$t=$s.TpmEnabled};if($f-and$f.SecureBoot-eq'On'){$b=$true};$e['Security']=@{TpmEnabled=$t;SecureBoot=$b}}else{$e['Security']=@{TpmEnabled=$false;SecureBoot=$false}};$e['HasCheckpoint']=[bool](Get-VMSnapshot -VMName $n -EA 0);$dd=@();foreach($d in(Get-VMHardDiskDrive -VMName $n -EA 0)){if(-not$d.Path){continue};$v=Get-VHD -Path $d.Path -EA 0;$c=0;$rc=$false;if($v){$c=$v.Size;if($v.RctId){$rc=$true}};$dd+=@{Path=$d.Path;Capacity=$c;RCTEnabled=$rc;CT=[int]$d.ControllerType;CN=$d.ControllerNumber;CL=$d.ControllerLocation}};$e['Disks']=$dd;$nn=@();foreach($a in(Get-VMNetworkAdapter -VMName $n -EA 0)){$vl=0;$vi=$a|Get-VMNetworkAdapterVlan -EA 0;if($vi-and$vi.AccessVlanId){$vl=$vi.AccessVlanId};$nn+=@{Name=$a.Name;MAC=$a.MacAddress;Switch=$a.SwitchName;Vlan=$vl}};$e['NICs']=$nn;if($vm.State-eq'Running'){$ci=Get-CimInstance -Namespace root\virtualization\v2 -ClassName Msvm_ComputerSystem -Filter "ElementName='$n'" -EA 0;if($ci){$kv=Get-CimAssociatedInstance -InputObject $ci -ResultClassName Msvm_KvpExchangeComponent -EA 0;if($kv-and$kv.GuestIntrinsicExchangeItems){$os='';$om='';foreach($i in $kv.GuestIntrinsicExchangeItems){$x=[xml]$i;$pn=$x.INSTANCE.PROPERTY|?{$_.NAME-eq'Name'}|Select -Exp VALUE;$pv=$x.INSTANCE.PROPERTY|?{$_.NAME-eq'Data'}|Select -Exp VALUE;if($pn-eq'OSName'){$os=$pv}elseif($pn-eq'OSMajorVersion'){$om=$pv}};if($os-and$om-and$os-notmatch'\d'){$os="$os $om"};$e['GuestOS']=$os};$vs=Get-CimAssociatedInstance -InputObject $ci -ResultClassName Msvm_VirtualSystemSettingData|?{$_.VirtualSystemType-eq'Microsoft:Hyper-V:System:Realized'};if($vs){$ps=Get-CimAssociatedInstance -InputObject $vs -ResultClassName Msvm_SyntheticEthernetPortSettingData;$nc=@();foreach($p in $ps){$gc=Get-CimAssociatedInstance -InputObject $p -ResultClassName Msvm_GuestNetworkAdapterConfiguration;if($gc){$nc+=[PSCustomObject]@{MAC=$p.Address;IPs=$gc.IPAddresses;Subnets=$gc.Subnets;DHCP=$gc.DHCPEnabled;GW=$gc.DefaultGateways;DNS=$gc.DNSServers}}};if($nc.Count-gt 0){$e['GuestNetworks']=$nc}}}};$r[$n]=$e};$r|ConvertTo-Json -Depth 4 -Compress`

	// BatchGetVMDetailsLight: disks/NICs/checkpoints only (staging), capacity/RCT/guest deferred.
	BatchGetVMDetailsLight = `$r=@{};foreach($vm in(Get-VM)){$n=$vm.Name;$e=@{};$e['Security']=@{TpmEnabled=$false;SecureBoot=$false};$e['HasCheckpoint']=[bool](Get-VMSnapshot -VMName $n -EA 0);$dd=@();foreach($d in(Get-VMHardDiskDrive -VMName $n -EA 0)){if(-not$d.Path){continue};$dd+=@{Path=$d.Path;Capacity=0;RCTEnabled=$false;CT=[int]$d.ControllerType;CN=$d.ControllerNumber;CL=$d.ControllerLocation}};$e['Disks']=$dd;$nn=@();foreach($a in(Get-VMNetworkAdapter -VMName $n -EA 0)){$vl=0;$vi=$a|Get-VMNetworkAdapterVlan -EA 0;if($vi-and$vi.AccessVlanId){$vl=$vi.AccessVlanId};$nn+=@{Name=$a.Name;MAC=$a.MacAddress;Switch=$a.SwitchName;Vlan=$vl}};$e['NICs']=$nn;$r[$n]=$e};$r|ConvertTo-Json -Depth 4 -Compress`

	// ListVMsWithDetailsLight: Get-VM + light details in one call → {"VMs":[],"Details":{}}.
	ListVMsWithDetailsLight = `$vms=@(Get-VM|Select-Object Id,Name,@{N='State';E={[int]$_.State}},ProcessorCount,MemoryStartup,Generation);$r=@{};foreach($vm in $vms){$n=$vm.Name;$e=@{};$e['Security']=@{TpmEnabled=$false;SecureBoot=$false};$e['HasCheckpoint']=[bool](Get-VMSnapshot -VMName $n -EA 0);$dd=@();foreach($d in(Get-VMHardDiskDrive -VMName $n -EA 0)){if(-not$d.Path){continue};$dd+=@{Path=$d.Path;Capacity=0;RCTEnabled=$false;CT=[int]$d.ControllerType;CN=$d.ControllerNumber;CL=$d.ControllerLocation}};$e['Disks']=$dd;$nn=@();foreach($a in(Get-VMNetworkAdapter -VMName $n -EA 0)){$vl=0;$vi=$a|Get-VMNetworkAdapterVlan -EA 0;if($vi-and$vi.AccessVlanId){$vl=$vi.AccessVlanId};$nn+=@{Name=$a.Name;MAC=$a.MacAddress;Switch=$a.SwitchName;Vlan=$vl}};$e['NICs']=$nn;$r[$n]=$e};@{VMs=$vms;Details=$r}|ConvertTo-Json -Depth 4 -Compress`

	// BatchGetVHDCapacity: path → {S,R} after LightMode load.
	BatchGetVHDCapacity = `$r=@{};Get-VM|Get-VMHardDiskDrive -EA 0|%{if($_.Path){$v=Get-VHD -Path $_.Path -EA 0;if($v){$r[$_.Path]=@{S=$v.Size;R=[bool]$v.RctId}}}};$r|ConvertTo-Json -Compress`

	// BatchGetVMHardware: hardware only (no guest).
	BatchGetVMHardware = `$r=@{};foreach($vm in(Get-VM)){$n=$vm.Name;$e=@{};if($vm.Generation-eq 2){$s=Get-VMSecurity -VMName $n -EA 0;$f=Get-VMFirmware -VMName $n -EA 0;$t=$false;$b=$false;if($s){$t=$s.TpmEnabled};if($f-and$f.SecureBoot-eq'On'){$b=$true};$e['Security']=@{TpmEnabled=$t;SecureBoot=$b}}else{$e['Security']=@{TpmEnabled=$false;SecureBoot=$false}};$e['HasCheckpoint']=[bool](Get-VMSnapshot -VMName $n -EA 0);$dd=@();foreach($d in(Get-VMHardDiskDrive -VMName $n -EA 0)){if(-not$d.Path){continue};$v=Get-VHD -Path $d.Path -EA 0;$c=0;$rc=$false;if($v){$c=$v.Size;if($v.RctId){$rc=$true}};$dd+=@{Path=$d.Path;Capacity=$c;RCTEnabled=$rc;CT=[int]$d.ControllerType;CN=$d.ControllerNumber;CL=$d.ControllerLocation}};$e['Disks']=$dd;$nn=@();foreach($a in(Get-VMNetworkAdapter -VMName $n -EA 0)){$vl=0;$vi=$a|Get-VMNetworkAdapterVlan -EA 0;if($vi-and$vi.AccessVlanId){$vl=$vi.AccessVlanId};$nn+=@{Name=$a.Name;MAC=$a.MacAddress;Switch=$a.SwitchName;Vlan=$vl}};$e['NICs']=$nn;$r[$n]=$e};$r|ConvertTo-Json -Depth 4 -Compress`

	// BatchGetVMGuest: guest OS + networks for running VMs.
	BatchGetVMGuest = `$r=@{};foreach($vm in(Get-VM|?{$_.State-eq'Running'})){$n=$vm.Name;$e=@{};$ci=Get-CimInstance -Namespace root\virtualization\v2 -ClassName Msvm_ComputerSystem -Filter "ElementName='$n'" -EA 0;if($ci){$kv=Get-CimAssociatedInstance -InputObject $ci -ResultClassName Msvm_KvpExchangeComponent -EA 0;if($kv-and$kv.GuestIntrinsicExchangeItems){$os='';$om='';foreach($i in $kv.GuestIntrinsicExchangeItems){$x=[xml]$i;$pn=$x.INSTANCE.PROPERTY|?{$_.NAME-eq'Name'}|Select -Exp VALUE;$pv=$x.INSTANCE.PROPERTY|?{$_.NAME-eq'Data'}|Select -Exp VALUE;if($pn-eq'OSName'){$os=$pv}elseif($pn-eq'OSMajorVersion'){$om=$pv}};if($os-and$om-and$os-notmatch'\d'){$os="$os $om"};$e['GuestOS']=$os};$vs=Get-CimAssociatedInstance -InputObject $ci -ResultClassName Msvm_VirtualSystemSettingData|?{$_.VirtualSystemType-eq'Microsoft:Hyper-V:System:Realized'};if($vs){$ps=Get-CimAssociatedInstance -InputObject $vs -ResultClassName Msvm_SyntheticEthernetPortSettingData;$nc=@();foreach($p in $ps){$gc=Get-CimAssociatedInstance -InputObject $p -ResultClassName Msvm_GuestNetworkAdapterConfiguration;if($gc){$nc+=[PSCustomObject]@{MAC=$p.Address;IPs=$gc.IPAddresses;Subnets=$gc.Subnets;DHCP=$gc.DHCPEnabled;GW=$gc.DefaultGateways;DNS=$gc.DNSServers}}};if($nc.Count-gt 0){$e['GuestNetworks']=$nc}}};if($e.Count-gt 0){$r[$n]=$e}};$r|ConvertTo-Json -Depth 4 -Compress`
)
