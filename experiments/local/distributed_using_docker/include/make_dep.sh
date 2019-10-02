#!/bin/bash

PROJECT_PATH=$1
PROJECT_GOPATH=$2
export GOPATH=$PROJECT_GOPATH

go get -v -d -t "${PROJECT_PATH}"/...
