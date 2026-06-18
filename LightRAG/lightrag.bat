@echo off
setlocal enabledelayedexpansion

REM Get the directory where the script is located
set SCRIPT_DIR=%~dp0
REM Remove trailing backslash
set SCRIPT_DIR=%SCRIPT_DIR:~0,-1%

REM Set default values for directories
set INPUT_DIR=
set WORKING_DIR=

REM Check if arguments are provided
if "%~1"=="" (
    REM No INPUT_DIR provided, use current directory where script was called
    set INPUT_DIR=%CD%
) else (
    set INPUT_DIR=%~1
)

if "%~2"=="" (
    REM No WORKING_DIR provided, create/use .lightrag in current directory
    set WORKING_DIR=%CD%\.lightrag
) else (
    set WORKING_DIR=%~2
)

REM Check if INPUT_DIR exists
if not exist "%INPUT_DIR%" (
    echo ERROR: INPUT_DIR does not exist: %INPUT_DIR%
    exit /b 1
)

REM Create WORKING_DIR if it doesn't exist
if not exist "%WORKING_DIR%" (
    echo Creating WORKING_DIR: %WORKING_DIR%
    mkdir "%WORKING_DIR%"
    if %ERRORLEVEL% NEQ 0 (
        echo ERROR: Failed to create WORKING_DIR: %WORKING_DIR%
        exit /b %ERRORLEVEL%
    )
)

REM Set environment variables for docker compose
set COMPOSE_PROJECT_NAME=lightrag
set INPUT_DIR=%INPUT_DIR%
set WORKING_DIR=%WORKING_DIR%

REM Navigate to the script directory where docker-compose.yml is located
cd /d "%SCRIPT_DIR%"

REM Create backup of docker-compose.yml
echo Creating backup of docker-compose.yml
copy docker-compose.yml docker-compose.yml.bak

REM Run docker compose up with environment variables
echo Starting LightRAG with INPUT_DIR=%INPUT_DIR% and WORKING_DIR=%WORKING_DIR%
docker compose -p lightrag up

REM Check if docker compose command succeeded
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: docker compose up failed with exit code %ERRORLEVEL%
    exit /b %ERRORLEVEL%
)

echo LightRAG started successfully