#! /bin/bash

echo update > setup.log
sudo apt update

echo upgrade >> setup.log
sudo apt -y upgrade

echo install gcc >> setup.log
sudo apt -y install gcc zip

echo install go from snap >> setup.log
sudo snap install go --classic

echo install dependencies >> setup.log
go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega/...
go get golang.org/x/crypto/...
go get golang.org/x/sync/semaphore
go get github.com/rs/zerolog

echo create gomel dir >> setup.log
mkdir -p go/src/gitlab.com/alephledger

echo done >> setup.log
