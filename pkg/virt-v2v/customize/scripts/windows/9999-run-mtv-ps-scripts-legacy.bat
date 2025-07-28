@echo off
setlocal enabledelayedexpansion

rem Set PowerShell 1.0 execution policy to Unrestricted (requires admin)
rem This is required to allow execution of unsigned PowerShell scripts during first boot.
rem By default, PowerShell may block .ps1 scripts (ExecutionPolicy = Restricted),
rem and setting it to Unrestricted ensures the scripts can run without being blocked.
reg add "HKLM\SOFTWARE\Microsoft\PowerShell\1\ShellIds\Microsoft.PowerShell" /v ExecutionPolicy /t REG_SZ /d Unrestricted /f

set firstboot=C:\Program Files\Guestfs\Firstboot
set scripts=%firstboot%\scripts
set scripts_done=%firstboot%\scripts-done

echo Running MTV first boot scripts

for %%f in ("%scripts%\*.ps1") do (
  echo running "%%~f"
  powershell.exe -Command "& '%%~f'"
  set elvl=!errorlevel!
  echo .... exit code !elvl!
  move "%%~f" "%scripts_done%"
)

rem Optionally reset policy to Restricted (requires admin)
rem This locks script execution back down after boot scripts are complete
reg add "HKLM\SOFTWARE\Microsoft\PowerShell\1\ShellIds\Microsoft.PowerShell" /v ExecutionPolicy /t REG_SZ /d Restricted /f