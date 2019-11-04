#!/bin/bash
set -e

PKG=$1
GOPATH=$2

go run ${PKG}/tests/online_localhost
