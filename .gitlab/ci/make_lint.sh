#!/bin/bash
#
# parameters: <required linter output file>

PKG=$1
LINTER_OUTPUT=$2

PKG_LIST=$(go list ${PKG}/... | grep -v /vendor/)

golint -set_exit_status ${PKG_LIST} | tee ${LINTER_OUTPUT}
exit "${PIPESTATUS[0]}
