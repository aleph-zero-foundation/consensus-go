#!/bin/bash

PKG=$1

go build ${PKG}/cmd/... && go build ${PKG}/pkg/...
