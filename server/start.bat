@echo off
rem local dev startup script (default 8080, arg overrides port)
rem usage: start.bat 18080
cd /d D:\hotel-management\server
set PORT=8080
if not "%~1"=="" set PORT=%~1
D:\go\bin\go.exe run .
