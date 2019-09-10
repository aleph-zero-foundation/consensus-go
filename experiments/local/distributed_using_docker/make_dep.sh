#!/bin/bash

PROJECT_PATH=$1
PROJECT_GOPATH=$2
export GOPATH=$PROJECT_GOPATH

go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega/...

go get -u github.com/golang/lint/golint

go get -v -d "${PROJECT_PATH}"/...
