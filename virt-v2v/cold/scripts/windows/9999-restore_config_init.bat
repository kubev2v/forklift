@echo off
echo Restore configuration disks
PowerShell -NoProfile -ExecutionPolicy Bypass -Command "\'Program Files'\Guestfs\Firstboot\Scripts\9999-restore_config.ps1"
