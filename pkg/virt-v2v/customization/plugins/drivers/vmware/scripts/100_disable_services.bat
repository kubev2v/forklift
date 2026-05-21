@echo off
:: ===============================================================
:: disable_vmware_srv.cmd
:: Permanently disable all VMware services
:: ===============================================================

setlocal enabledelayedexpansion

REM =============================
REM Check for admin privileges
REM =============================
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo ERROR: This script must be run as Administrator!
    echo Please right-click and select "Run as administrator"
    pause
    exit /b 1
)

:: --- Script directory ---
set "SCRIPT_DIR=%~dp0"
set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"

:: --- Log file ---
set "LOG=%SCRIPT_DIR%\vmware_disable.log"
echo =============================================================== > "%LOG%"
echo   VMware Service Disable Log - %DATE% %TIME% >> "%LOG%"
echo =============================================================== >> "%LOG%"

echo.
echo ===============================================================
echo   VMware Service Disabler (Run as Administrator)
echo ===============================================================
echo.
echo [INFO] Log file: %LOG%
echo.

:: ===============================================================
:: Enumerate all services
:: ===============================================================
echo [INFO] Enumerating services...
set "TEMPFILE=%temp%\all_services.txt"
sc query type= service state= all > "%TEMPFILE%"

:: Initialize
set "VMWARE_LIST="
set /a MATCH_COUNT=0

:: ===============================================================
:: Check each service for VMware
:: ===============================================================
for /f "tokens=2 delims=:" %%A in ('findstr /R "^SERVICE_NAME" "%TEMPFILE%"') do (
    set "SVC=%%A"
    set "SVC=!SVC: =!"  :: Remove leading space
    call :CheckVMwareService !SVC!
)

del "%TEMPFILE%" >nul 2>&1

if %MATCH_COUNT%==0 (
    echo [INFO] No VMware-related services found. >> "%LOG%"
    echo [INFO] No VMware-related services found.
    goto :EOF
)

echo.
echo [INFO] Found %MATCH_COUNT% VMware-related services:
for %%S in (!VMWARE_LIST!) do echo   %%S
echo.

REM choice /m "Proceed to stop and disable these services permanently?"
REM if errorlevel 2 (
REM    echo [INFO] Skipping services. >> "%LOG%"
REM    goto :EOF
REM )

:: ===============================================================
:: Stop and disable VMware services
:: ===============================================================
for %%S in (!VMWARE_LIST!) do (
    call :StopAndDisableService "%%S"
)

echo.
echo ===============================================================
echo VMware services have been processed.
echo See log for details: %LOG%
echo ===============================================================
goto :EOF

:: ===============================================================
:: SUBROUTINES
:: ===============================================================

:CheckVMwareService
setlocal
set "SVC=%~1"
set "IS_VMWARE="

:: Query service configuration
for /f "tokens=*" %%L in ('sc qc "%SVC%" 2^>nul') do (
    echo %%L | findstr /I "VMware" >nul && set "IS_VMWARE=1"
)

if defined IS_VMWARE (
    echo [MATCH] %SVC% appears to be VMware-related.
    echo [MATCH] %SVC% appears to be VMware-related. >> "%LOG%"
    endlocal & set "VMWARE_LIST=%VMWARE_LIST% %SVC%" & set /a MATCH_COUNT+=1
) else (
    endlocal
)
goto :eof

:StopAndDisableService
setlocal
set "SVC=%~1"
echo ---------------------------------------------------------------
echo [ACTION] Stopping and disabling %SVC%...
echo [ACTION] Stopping and disabling %SVC%... >> "%LOG%"

:: Stop gracefully
sc stop "%SVC%" >nul 2>&1

:: Wait a few seconds
ping 127.0.0.1 -n 3 >nul

:: Force kill if still running
for /f "tokens=2 delims=:" %%P in ('sc queryex "%SVC%" ^| find "PID"') do (
    set "PID=%%P"
    set "PID=!PID: =!"
    if not "!PID!"=="0" (
        echo [ACTION] Forcibly killing PID !PID!
        echo [ACTION] Forcibly killing PID !PID! >> "%LOG%"
        taskkill /PID !PID! /F >nul 2>&1
    )
)

:: Disable permanently
sc config "%SVC%" start= disabled >nul 2>&1

:: Show final state
sc query "%SVC%" | findstr /I "STATE"

echo [INFO] %SVC% processed. >> "%LOG%"
endlocal
goto :eof
