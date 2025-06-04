@echo off
setlocal enabledelayedexpansion

rem Set PowerShell 1.0 execution policy to Unrestricted (requires admin)
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
reg add "HKLM\SOFTWARE\Microsoft\PowerShell\1\ShellIds\Microsoft.PowerShell" /v ExecutionPolicy /t REG_SZ /d Restricted /f