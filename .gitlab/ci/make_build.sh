#!/bin/bash

PKG=$1

go build ${PKG}/... && \
    find . -iname "*_test.go" -print0 | xargs -0 -n1 go test -c
