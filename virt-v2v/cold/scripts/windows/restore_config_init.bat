@echo off
echo Restore configuration disks
PowerShell -NoProfile -ExecutionPolicy Bypass -Command "\'Program Files'\Guestfs\Firstboot\restore_config.ps1"
