@ECHO OFF
SETLOCAL
SET GO111MODULE=on
SET GOFLAGS=-mod=vendor
SET GOOS=windows
SET GOARCH=amd64
PUSHD "%~dp0..\src"
GO.EXE build -i -o ..\bin\ServerWatchdog.exe .
SET GOOS=linux
GO.EXE build -i -o ..\bin\serverwatchdog .
POPD
ENDLOCAL
