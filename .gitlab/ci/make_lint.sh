#!/bin/bash

PKG_LIST=$(go list ${PKG}/... | grep -v /vendor/)

golint -set_exit_status ${PKG_LIST} | tee $1
exit "${PIPESTATUS[0]}
