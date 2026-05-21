@echo off
:: ===============================================================
:: remove_vmware_driver_packages.cmd
:: Remove all VMware-related driver packages using pnputil.exe
:: Enumerates all drivers, filters for VMware providers, and removes them
:: More aggressive than remove_vmware_drivers.bat - removes driver packages
:: ===============================================================

setlocal enabledelayedexpansion

REM =============================
REM Check for admin privileges
REM =============================
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo ERROR: This script must be run as Administrator!
    echo Please right-click and select "Run as administrator"
REM    pause
    exit /b 1
)

:: --- Get script directory ---
set "SCRIPT_DIR=%~dp0"
set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"

echo.
echo ===============================================================
echo   Remove VMware Driver Packages Script
echo   Run as Administrator
echo ===============================================================
echo.

echo ==========================================
echo  Searching for VMware drivers and packages
echo ==========================================
echo.

:: --- Enumerate all drivers ---
pnputil /enum-drivers > "%temp%\all_drivers.txt"

:: --- Filter for VMware drivers ---
echo Filtering lines with Published Name and VMware...
findstr /i /c:"Published Name" /c:"Provider Name" "%temp%\all_drivers.txt" > "%temp%\vmware_drivers.txt"

echo.
echo ===== VMware Drivers Found =====

:: --- Initialize variables ---
set COUNT=0
set LAST_PUBLISHED=

:: --- Parse driver list for VMware providers ---
for /f "tokens=1,* delims=:" %%A in (%temp%\vmware_drivers.txt) do (
    set LINE=%%A
    set VALUE=%%B
    set VALUE=!VALUE: =!

    if /i "!LINE!"=="Published Name" (
        set LAST_PUBLISHED=!VALUE!
    )

    if /i "!LINE!"=="Provider Name" (
        echo !VALUE! | findstr /i "VMware" >nul
        if !errorlevel! == 0 (
            REM This Published Name belongs to VMware
            if not "!LAST_PUBLISHED!"=="" (
                echo Found VMware INF: !LAST_PUBLISHED!
                set INF_LIST[!COUNT!]=!LAST_PUBLISHED!
                set /a COUNT+=1
            )
        )
        set LAST_PUBLISHED=
    )
)

:: --- Check if any drivers were found ---
if %COUNT% EQU 0 (
    echo No VMware drivers found to remove.
    echo.
    echo ===============================================================
    echo No VMware driver packages found.
    echo ===============================================================
REM    pause
    exit /b 0
)

echo.
echo ===============================
echo  Removing VMware Driver Packages
echo ===============================
echo.

:: --- Remove each VMware driver package ---
set "LOG_FILE=%SCRIPT_DIR%\removal_log.txt"
set "REBOOT_REQUIRED=0"
for /l %%I in (0,1,%COUNT%-1) do (
    set INF=!INF_LIST[%%I]!
    if not "!INF!"=="" (
        echo Removing !INF! ...
        pnputil /delete-driver "!INF!" /uninstall /force > "%temp%\pnputil_output.txt" 2>&1
        type "%temp%\pnputil_output.txt" >> "%LOG_FILE%"
        type "%temp%\pnputil_output.txt" | findstr /i "reboot restart" >nul
        if !errorlevel! == 0 (
            set REBOOT_REQUIRED=1
        )
        echo Done.
    )
)
del "%temp%\pnputil_output.txt" >nul 2>&1

:: --- Clean up temporary files ---
del "%temp%\all_drivers.txt" >nul 2>&1
del "%temp%\vmware_drivers.txt" >nul 2>&1

echo.
echo ===============================================================
echo  VMware driver removal process finished
echo  Log saved to: %LOG_FILE%
if %REBOOT_REQUIRED% EQU 1 (
    echo.
    echo WARNING: A system reboot is required to complete the removal.
    echo Please restart your computer when convenient.
)
echo ===============================================================
REM pause
exit /b 0
