@echo off
rem 本地开发启动脚本（默认 8080，可传参指定端口）
rem 用法: start.bat 18080
cd /d D:\hotel-management\server
set PORT=8080
if not "%~1"=="" set PORT=%~1
D:\go\bin\go.exe run .
