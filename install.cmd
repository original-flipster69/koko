@echo off
setlocal enabledelayedexpansion

:: koko installer for Windows
:: Usage: install.cmd [--version <version>] [--mirror <mirror>] [--help]

set "REPO=original-flipster69/koko"
set "INSTALL_DIR=%~dp0"
set "BINARY_NAME=koko"
set "MIRROR=codeberg"

:: Parse command line arguments
:parse_args
if "%~1"=="" goto end_parse
if /i "%~1"=="--version" (
    set "KOKO_VERSION=%~2"
    shift
    shift
    goto parse_args
)
if /i "%~1"=="--mirror" (
    set "MIRROR=%~2"
    shift
    shift
    goto parse_args
)
if /i "%~1"=="--help" (
    goto usage
)
if /i "%~1"=="-h" (
    goto usage
)
echo Error: Unknown option: %~1
goto usage

:end_parse

:: Usage information
:usage
echo Usage: %~nx0 [OPTIONS]
echo.
echo Options:
echo   --version ^<version^>    Install a specific version (default: latest)
echo   --mirror ^<mirror^>      Select mirror: github or codeberg (default: codeberg)
echo   --help                 Show this help message
exit /b 1

:: Check for required tools
where curl >nul 2>&1
if errorlevel 1 (
    echo Error: 'curl' is required
    exit /b 1
)

where tar >nul 2>&1
if errorlevel 1 (
    echo Error: 'tar' is required
    exit /b 1
)

:: Detect platform
set "OS=windows"
set "ARCH=%PROCESSOR_ARCHITECTURE%"

if "%ARCH%"=="AMD64" (
    set "ARCH=amd64"
) else if "%ARCH%"=="x86" (
    set "ARCH=386"
) else if "%ARCH%"=="ARM64" (
    set "ARCH=arm64"
) else (
    echo Error: unsupported architecture: %ARCH%
    exit /b 1
)

:: Fetch latest tag if version not specified
if defined KOKO_VERSION (
    set "TAG=%KOKO_VERSION%"
) else (
    :: Determine the base URL based on the mirror
    if "%MIRROR%"=="github" (
        set "BASE_URL=https://github.com/%REPO%/releases/latest"
    ) else (
        set "BASE_URL=https://codeberg.org/%REPO%/releases/latest"
    )
    
    :: Use curl to get the latest tag
    for /f "delims=" %%i in ('curl -fsSLI -o nul -w "%%{url_effective}" "!BASE_URL!"') do set "url=%%i"
    
    :: Extract tag from URL
    for %%i in (!url!) do (
        set "TAG=%%~nxi"
        set "TAG=!TAG:tag/=!"
    )
    
    :: Ensure tag starts with 'v'
    echo !TAG! | findstr /b "v" >nul
    if errorlevel 1 (
        echo Error: unexpected release URL: !url!
        exit /b 1
    )
)

:: Download and verify
set "VERSION=!TAG:v=!"
set "ASSET=%BINARY_NAME%_%VERSION%_%OS%_%ARCH%.zip"

:: Determine the base URL based on the mirror
if "%MIRROR%"=="github" (
    set "ASSET_URL=https://github.com/%REPO%/releases/download/%TAG%/%ASSET%"
    set "CHECKSUMS_URL=https://github.com/%REPO%/releases/download/%TAG%/checksums.txt"
) else (
    set "ASSET_URL=https://codeberg.org/%REPO%/releases/download/%TAG%/%ASSET%"
    set "CHECKSUMS_URL=https://codeberg.org/%REPO%/releases/download/%TAG%/checksums.txt"
)

:: Create temp directory
set "TMPDIR=%TEMP%\koko_install_%RANDOM%"
mkdir "%TMPDIR%"

:: Download asset and checksums
echo Downloading %ASSET% (%TAG%) from %MIRROR%...
curl -fsSL -o "%TMPDIR%\%ASSET%" "%ASSET_URL%"
if errorlevel 1 (
    echo Error: failed to download %ASSET%
    rmdir /s /q "%TMPDIR%"
    exit /b 1
)

curl -fsSL -o "%TMPDIR%\checksums.txt" "%CHECKSUMS_URL%"
if errorlevel 1 (
    echo Error: failed to download checksums.txt
    rmdir /s /q "%TMPDIR%"
    exit /b 1
)

:: Verify checksum
for /f "tokens=1,2 delims= " %%a in ('findstr /b /c:"%ASSET%" "%TMPDIR%\checksums.txt"') do (
    set "EXPECTED=%%a"
    set "ASSET_FILE=%%b"
)

if not defined EXPECTED (
    echo Error: %ASSET% not listed in checksums.txt
    rmdir /s /q "%TMPDIR%"
    exit /b 1
)

:: Compute SHA-256 of downloaded file
for /f "delims=" %%i in ('certutil -hashfile "%TMPDIR%\%ASSET%" SHA256 ^| findstr /v "SHA256"') do (
    set "ACTUAL=%%i"
    set "ACTUAL=!ACTUAL: =!"
)

if "!EXPECTED!" neq "!ACTUAL!" (
    echo Error: checksum mismatch for %ASSET%
    echo   expected: !EXPECTED!
    echo   actual:   !ACTUAL!
    rmdir /s /q "%TMPDIR%"
    exit /b 1
)
echo Checksum verified.

:: Extract the archive
tar -xf "%TMPDIR%\%ASSET%" -C "%TMPDIR%"
if errorlevel 1 (
    echo Error: failed to extract %ASSET%
    rmdir /s /q "%TMPDIR%"
    exit /b 1
)

:: Install binary
set "BINARY_FILE=%BINARY_NAME%.exe"

:: Check if install directory is writable
if exist "%INSTALL_DIR%\%BINARY_FILE%" (
    echo Installing to %INSTALL_DIR% (requires admin privileges)...
    :: Try to copy with admin privileges
    powershell -command "Start-Process cmd -ArgumentList '/c copy /Y \"%TMPDIR%\%BINARY_FILE%\" \"%INSTALL_DIR%\\%BINARY_FILE%\"' -Verb RunAs -Wait"
) else (
    copy /Y "%TMPDIR%\%BINARY_FILE%" "%INSTALL_DIR%\%BINARY_FILE%" >nul
)

:: Install config
set "CONFIG_DIR=%USERPROFILE%\.koko"
set "CONFIG_FILE=%CONFIG_DIR%\config.toml"

if exist "%CONFIG_FILE%" (
    echo Keeping existing config at %CONFIG_FILE%.
) else (
    :: Determine the base URL based on the mirror
    if "%MIRROR%"=="github" (
        set "BLUEPRINT_URL=https://raw.githubusercontent.com/%REPO%/%TAG%/docs/examples/config.toml"
    ) else (
        set "BLUEPRINT_URL=https://codeberg.org/%REPO%/raw/tag/%TAG%/docs/examples/config.toml"
    )
    
    mkdir "%CONFIG_DIR%" 2>nul
    curl -fsSL -o "%CONFIG_FILE%" "%BLUEPRINT_URL%"
    if errorlevel 1 (
        echo Warning: could not fetch starter config (skipping).
    ) else (
        echo Wrote starter config to %CONFIG_FILE%.
    )
)

:: Cleanup
rmdir /s /q "%TMPDIR%"

echo.
echo Installed %BINARY_NAME% %TAG% to %INSTALL_DIR%\%BINARY_FILE%
echo Run '%BINARY_FILE%' to start a session.
