@echo off
setlocal

set VERSION=0.0.1-dev
set OUTPUT=src\webdav-drive.exe

echo [1/2] Building...
go build -ldflags="-H windowsgui -s -w -X main.version=%VERSION%" -o "%OUTPUT%" ./src

echo [2/2] Finish!
echo   App: %OUTPUT%