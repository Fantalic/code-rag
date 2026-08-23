@echo off
rem codeRAG -- cmd.exe entry point for the interactive setup.
rem Thin wrapper: delegates to setup.ps1 with execution policy bypassed,
rem so cmd users don't hit "running scripts is disabled".
rem Flags are forwarded, e.g.:  scripts\setup.cmd -CI
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0setup.ps1" %*
exit /b %errorlevel%
