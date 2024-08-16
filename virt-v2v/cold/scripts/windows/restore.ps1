<#
.SYNOPSIS
  This script extracts the network adapters IP configuration when
  configuration is static, as well as the drives letters. This configuration
  is written is a script that will be run during next boot. It also installs
  RHV-APT if it is absent.

.DESCRIPTION
  When migrating a Windows virtual machine from VMware to KVM, the drivers
  are modified by virt-v2v. The consequence is that the new network adapters
  are created with the same MAC addresses as the original network adapters.
  With Windows, the new network adapters IP configuration is not bound to
  the MAC addresses, so the new network adapters are not configured, so the
  virtual machine is unreachable.

  Another potential hiccup is that drives may be ordered differently after the
  migration. This may lead to having wrong drive letters and programs not being
  able to find their data.

  We also have noticed that sometimes RHV-APT is not installed after the
  migration, even though virt-v2v first boot scripts have run successfully.
  The consequence is that, even though the VirtIO Win ISO image is attached to
  the virtual machine, the additional drivers and the RHV Guest Agent will not
  be installed.

  This script collects the configuration of the Windows virtual machine and
  adds commands to a new script, in order to:

    1. Disable the original network adapters to avoid conflict after migration.
    2. Configure the new network adapters based on MAC address.
    3. Configure the drives letters based on WMI object id.
    4. Install and start RHV-APT if it is absent.

  In order to run the generated script, we also create a batch file (.bat)
  under C:\Program Files\Guestfs\Firstboot\scripts, so that virt-v2v first boot
  script runs it. This avoids creating another scheduled task.

.NOTES
  Version:        1.0
  Author:         Fabien Dupont <fdupont@redhat.com>
  Purpose/Change: Extract system configuration during premigration
#>

$scriptDir = "C:\Program Files\Guestfs\Firstboot\scripts"
$restoreScriptFile = $scriptDir + "\9999-restore_config.ps1"
$firstbootScriptFile = $scriptDir + "\9999-restore_config.bat"

# Create the scripts folder if it does not exist
if (!(Get-Item $scriptDir -ErrorAction SilentlyContinue)) {
    New-Item -Type directory -Path $scriptDir
}

# Initialize the generated script
Write-Output ("# Migration - Reconfigure disks") > $restoreScriptFile
Write-Output ("") >> $restoreScriptFile

# Initialize the log file created by the generated script
Write-Output ("`$logFile = `$env:SystemDrive + '\Program Files\Guestfs\Firstboot\scripts-done\9999-restore_config.txt'") >> $restoreScriptFile
Write-Output ("Write-Output ('Starting restore_config.ps1 script') > `$logFile") >> $restoreScriptFile
Write-Output ("Write-Output ('') >> `$logFile") >> $restoreScriptFile
Write-Output ("") >> $restoreScriptFile

# Generate the script section to re-enable all offline drives
Write-Output ("# Re-enable all offline drives") >> $restoreScriptFile
Write-Output ("Write-Output ('Re-enabling all offline drives') >> `$logFile") >> $restoreScriptFile
Write-Output ("Get-Disk | Where { `$_.OperationalStatus -like 'Offline' } | % {") >> $restoreScriptFile
Write-Output ("     Write-Output ('  - ' + `$_.Number + ': ' + `$_.FriendlyName + '(' + [math]::Round(`$_.Size/1GB,2) + 'GB)') >> `$logFile") >> $restoreScriptFile
Write-Output ("    `$_ | Set-Disk -IsOffline `$false") >> $restoreScriptFile
Write-Output ("    `$_ | Set-Disk -IsReadOnly `$false") >> $restoreScriptFile
Write-Output ("}") >> $restoreScriptFile
Write-Output ("Write-Output ('') >> `$logFile") >> $restoreScriptFile
Write-Output ("") >> $restoreScriptFile

# Generate the script section to remove the access path on all partitions
# but SystemDrive
Write-Output ("# Remove the access path on all partitions but SystemDrive") >> $restoreScriptFile
Write-Output ("`$a = (Get-Item env:SystemDrive).Value.substring(0,1)") >> $restoreScriptFile
Write-Output ("Write-Output('Remove the partition access path on all partitions but SystemDrive (" + $a +")') >> `$logFile") >> $restoreScriptFile
Write-Output ("Get-Partition | Where { `$_.DriveLetter -notlike `$a -and `$_.DriveLetter.length -gt 0 } | % {") >> $restoreScriptFile
Write-Output ("    if ([string]::IsNullOrWhiteSpace(`$_.DriveLetter)) {") >> $restoreScriptFile
Write-Output ("        Write-Output ('  - DiskNumber: ' + `$_.DiskNumber + ' - PartitionNumber: ' + `$_.PartitionNumber + ' - No AccessPath. Skipping') >> `$logFile") >> $restoreScriptFile
Write-Output ("    }") >> $restoreScriptFile
Write-Output ("    else {") >> $restoreScriptFile
Write-Output ("        Write-Output ('  - DiskNumber: ' + `$_.DiskNumber + ' - PartitionNumber: ' + `$_.PartitionNumber + ' - AccessPath: ' + `$_.DriveLetter + ':') >> `$logFile") >> $restoreScriptFile
Write-Output ("        Remove-PartitionAccessPath -DiskNumber `$_.DiskNumber -PartitionNumber `$_.PartitionNumber -AccessPath (`$_.DriveLetter + ':')") >> $restoreScriptFile
Write-Output ("    }") >> $restoreScriptFile
Write-Output ("}") >> $restoreScriptFile
Write-Output ("Write-Output ('') >> `$logFile") >> $restoreScriptFile
Write-Output ("") >> $restoreScriptFile

# Generate the script section to restore the drive letters
Write-Output ("# Restore drive letters") >> $restoreScriptFile
Write-Output ("Write-Output ('Restore drive letters') >> `$logFile") >> $restoreScriptFile
Get-Volume | ForEach-Object {
    if ($_.FileSystemLabel -ne "System Reserved") {
        if ([string]::IsNullOrWhiteSpace($_.DriveLetter)) {
            Write-Output ("Write-Output ('  - DeviceId: " + $_.ObjectId + " - No DriveLetter. Skipping.') >> `$logFile") >> $restoreScriptFile
        }
        else {
            Write-Output ("Write-Output ('  - DeviceId: " + $_.ObjectId + " - DriveLetter: " + $_.DriveLetter + ":') >> `$logFile") >> $restoreScriptFile
            $escObjectId = $_.ObjectId -replace "\\", "\\"
            Write-Output ("`$wmiObject = Get-WmiObject -Class Win32_Volume -Filter `"DeviceId='" + $escObjectId + "'`"") >> $restoreScriptFile
            Write-Output ("`$wmiObject.DriveLetter = '" + $_.DriveLetter + ":'") >> $restoreScriptFile
            Write-Output ("`$wmiObject.Put()") >> $restoreScriptFile
            Write-Output ("") >> $restoreScriptFile
        }
    }
}
Write-Output ("Write-Output ('') >> `$logFile") >> $restoreScriptFile
Write-Output ("") >> $restoreScriptFile

# Generate the batch script that will be run by virt-v2v first boot script
# Using Out-File instead of Write-Output to set encoding and avoid UTF-8 BOM
'@echo off' | Out-File -FilePath $firstbootScriptFile -Encoding ascii
'' | Out-File -FilePath $firstbootScriptFile -Encoding ascii -Append
'echo Restore configuration for network adapters and disks' | Out-File -FilePath $firstbootScriptFile -Encoding ascii -Append
'PowerShell.exe -ExecutionPolicy ByPass -File "' + $restoreScriptFile + '"' | Out-File -FilePath $firstbootScriptFile -Encoding ascii -Append
