#!/bin/bash

PKG_LIST=$(go list ${PKG}/... | grep -v /vendor/)

go test -short ${PKG_LIST}
