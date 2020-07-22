#!/bin/bash

#get the script's directory
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do # resolve $SOURCE until the file is no longer a symlink
  DIR="$( cd -P "$( dirname "$SOURCE" )" >/dev/null 2>&1 && pwd )"
  SOURCE="$(readlink "$SOURCE")"
  [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE" # if $SOURCE was a relative symlink, we need to resolve it relative to the path where the symlink file was located
done
BASE_DIR="$( cd -P "$( dirname "$SOURCE" )" >/dev/null 2>&1 && pwd )"

#save original directory
ORIG_DIR=$(pwd)

#set some GO options
GO111MODULE=on
GOFLAGS=-mod=vendor

#compile
cd $BASE_DIR/../src
go build -i -o ../bin/server_watchdog .
cd $ORIG_DIR
