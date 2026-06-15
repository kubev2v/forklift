# Uncomment this line for lots of debug output.
# Set-PSDebug -Trace 2
Write-Host Installing QEMU Guest Agent
$msi = "C:\Program Files\Guestfs\Firstboot\Temp\qemu-ga-x86_64.msi"
$logfile = "$msi.log"
$maxAttempts = 10
$retryDelay = 60
for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
    Write-Host "Attempt $attempt of $maxAttempts"
    Write-Host "Writing log to $logfile"
    $proc = Start-Process -Wait -PassThru -FilePath "$msi" -ArgumentList "/norestart","/qn","/l+*vx","`"$logfile`""
    $exitCode = $proc.ExitCode
    Write-Host "Exit code: $exitCode"
    if ($exitCode -eq 0) {
        Write-Host "QEMU Guest Agent installed successfully"
        break
    }
    if ($attempt -lt $maxAttempts) {
        Write-Host "Install failed, retrying in $retryDelay seconds..."
        Start-Sleep -Seconds $retryDelay
    } else {
        Write-Host "Install failed after $maxAttempts attempts"
    }
}
