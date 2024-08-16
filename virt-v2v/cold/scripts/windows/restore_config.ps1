# Migration - Reconfigure disks
# Initialize the log file created by the generated script
$logFile = $env:SystemDrive + '\Program Files\Guestfs\Firstboot\scripts-done\9999-restore_config.txt'
Write-Output ('Starting restore_config.ps1 script') > $logFile
Write-Output ('') >> $logFile

# script section to re-enable all offline drives
# Re-enable all offline drives
Write-Output ('Re-enabling all offline drives') >> $logFile
Get-Disk | Where { $_.OperationalStatus -like 'Offline' } | % {
     Write-Output ('  - ' + $_.Number + ': ' + $_.FriendlyName + '(' + [math]::Round($_.Size/1GB,2) + 'GB)') >> $logFile
    $_ | Set-Disk -IsOffline $false
    $_ | Set-Disk -IsReadOnly $false
}
Write-Output ('') >> $logFile

# script section to restore the drive letters
# Restore drive letters
Write-Output ('Restore drive letters') >> $logFile
Get-Volume | ForEach-Object {
	$driveLetter = $_.DriveLetter
	if ( $_.FileSystemLabel -eq "System Reserved") {
        Write-Output ("  - DeviceId: " + $_.ObjectId + " - System Reserved. Skipping.") >> $logFile
		return
	}
	if ( -not $driveLetter ) {
        Write-Output ("  - DeviceId: " + $_.ObjectId + " - No DriveLetter. Skipping.") >> $logFile
		return
	}
	if ($_.ObjectID -match '(Volume\{.*\})') {
        Write-Output ("  - DeviceId: " + $_.ObjectId + " - DriveLetter: " + $_.DriveLetter + ":") >> $logFile
		$id = $Matches.1
		$wmiInstance = Get-WmiObject -Class Win32_Volume | Where-Object { $_.DeviceId -like "*$id*" } |
			Set-WmiInstance -Arguments @{DriveLetter="${driveLetter}:" }
	}
}
