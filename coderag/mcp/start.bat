@echo off
REM Change directory to the location of this batch file (optional if needed)
cd /d "%~dp0"

REM Run python module with passed arguments
python -m coderag_mcp %*

REM Pause is optional for debugging; remove or comment out if not needed
REM pause
