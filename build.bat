@echo off
setlocal enabledelayedexpansion

:: Create output directory
if not exist "build" mkdir build

:: Set Go environment variables for cross-compilation
set GO111MODULE=on

:: Windows builds
echo Building for Windows...
set GOOS=windows

:: Windows AMD64
set GOARCH=amd64
go build -o build/prism_windows_amd64.exe prism.go
echo Built: build/prism_windows_amd64.exe

:: Linux builds
echo Building for Linux...
set GOOS=linux

:: Linux AMD64
set GOARCH=amd64
go build -o build/prism_linux_amd64 prism.go
echo Built: build/prism_linux_amd64

:: Linux ARM64
set GOARCH=arm64
go build -o build/prism_linux_arm64 prism.go
echo Built: build/prism_linux_arm64

echo.
echo Build completed! All binaries are in the build directory:
dir build 