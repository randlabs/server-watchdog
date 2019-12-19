@ECHO OFF
SETLOCAL
SET GO111MODULE=on
SET GOFLAGS=-mod=vendor
PUSHD "%~dp0..\src"
GO.EXE build -i -o ..\bin\ServerWatchdog.exe .
POPD
ENDLOCAL
