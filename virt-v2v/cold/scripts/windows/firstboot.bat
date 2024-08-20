@echo off

setlocal EnableDelayedExpansion
set firstboot=C:\Program Files\Guestfs\Firstboot
set log=%firstboot%\log.txt

set scripts=%firstboot%\scripts
set scripts_done=%firstboot%\scripts-done

call :main >> "%log%" 2>&1
exit /b

:main
echo starting firstboot service

if not exist "%scripts_done%" (
  mkdir "%scripts_done%"
)

:: Pick the next script to run.
for %%f in ("%scripts%"\*.bat) do (
  echo running "%%f"
  move "%%f" "%scripts_done%"
  pushd "%scripts_done%"
  call "%%~nf"
  set elvl=!errorlevel!
  echo .... exit code !elvl!
  popd

  :: Reboot the computer.  This is necessary to free any locked
  :: files which may prevent later scripts from running.
  shutdown /r /t 0 /y

  :: Exit the script (in case shutdown returns before rebooting).
  :: On next boot, the whole firstboot service will be called again.
  exit /b
)

:: Fallthrough here if there are no scripts.
echo uninstalling firstboot service
"%firstboot%\rhsrvany.exe" -s firstboot uninstall
