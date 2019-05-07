#!/bin/bash

PKG=$1

PKG_LIST=$(go list ${PKG}/... | grep -v /vendor/)

go test -short ${PKG_LIST}
