@echo off
setlocal

set VERSION=0.0.5
set OUTPUT=src\webdav-drive.exe
set INSTALLER_SCRIPT=installer\webdav-drive.iss

echo [+] Building v%VERSION%...
go build -ldflags="-H windowsgui -s -w -X main.version=%VERSION%" -o "%OUTPUT%" ./src
if errorlevel 1 (
    echo [-] Compilation failed.
    pause
    exit /b 1
)

echo [+] Creating installer...
iscc "%INSTALLER_SCRIPT%"
if errorlevel 1 (
    echo [-] Installer creation failed.
    pause
    exit /b 1
)

echo [+] Installer ready in 'installer' folder.