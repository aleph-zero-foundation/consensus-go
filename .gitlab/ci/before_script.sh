#!/bin/bash

mkdir -p ${GOPATH}/src/${MAIN_FOLDER} ${GOPATH}/src/_/builds
cp -r ${CI_PROJECT_DIR} ${GOPATH}/src/${PKG}
ln -s ${GOPATH}/src/${MAIN_FOLDER} ${GOPATH}/src/_/builds/${VENDOR_NAME}
chmod +x .gitlab/ci/make_*.sh
.gitlab/ci/make_dep.sh
