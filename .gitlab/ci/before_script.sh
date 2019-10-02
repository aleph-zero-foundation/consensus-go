#!/bin/bash
set -e

GOPATH=$1
MAIN_FOLDER=$2
PKG=$3
VENDOR_NAME=$4
CI_PROJECT_DIR=$5

mkdir -p ${GOPATH}/src/${MAIN_FOLDER} ${GOPATH}/src/_/builds
cp -r ${CI_PROJECT_DIR} ${GOPATH}/src/${PKG}
ln -s ${GOPATH}/src/${MAIN_FOLDER} ${GOPATH}/src/_/builds/${VENDOR_NAME}
chmod +x .gitlab/ci/make_*.sh
.gitlab/ci/make_dep.sh
