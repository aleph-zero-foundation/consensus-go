#!/bin/bash

go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega/...

go get -u github.com/golang/lint/golint

go get -v -d ./...
