@echo off
setlocal

set VERSION=0.0.5
set EXE_NAME=webdav-drive.exe
set BUILD_DIR=src

echo [+] Building WebDAV Drive v%VERSION%...

pushd "%BUILD_DIR%"
go build -ldflags="-s -w -X main.version=%VERSION%" -o "%EXE_NAME%" .
popd

if errorlevel 1 (
    echo [-] Build failed.
    pause
    exit /b 1
)

echo [+] Launching application...
start "" "%BUILD_DIR%\%EXE_NAME%"


echo [+] Done.