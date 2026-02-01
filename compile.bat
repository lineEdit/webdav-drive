@echo off
setlocal

set VERSION=0.0.4
set OUTPUT=src\webdav-drive.exe

echo Building v%VERSION%...
go build -ldflags="-H windowsgui -s -w -X main.version=%VERSION%" -o "%OUTPUT%" ./src
if errorlevel 1 exit /b 1

echo Creating installer...
iscc "installer\webdav-drive.iss"
if errorlevel 1 exit /b 1

echo Build completed.