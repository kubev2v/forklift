# Migration - Reconfigure disks
# Initialize the log file created by the generated script
$logFile = $env:SystemDrive + '\Program Files\Guestfs\Firstboot\scripts-done\9999-restore_config.txt'
Write-Output ('Starting restore_config.ps1 script') > $logFile
Write-Output ('') >> $logFile

# script section to re-enable all offline drives
# Re-enable all offline drives
Write-Output ('Re-enabling all offline drives') >> $logFile
Get-Disk | Where { $_.FriendlyName -like '*VirtIO*' } | % {
     Write-Output ('  - ' + $_.Number + ': ' + $_.FriendlyName + '(' + [math]::Round($_.Size/1GB,2) + 'GB)') >> $logFile
    $_ | Set-Disk -IsOffline $false
    $_ | Set-Disk -IsReadOnly $false
}
Write-Output ('') >> $logFile
