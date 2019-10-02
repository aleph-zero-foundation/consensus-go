#!/bin/bash

PID=$(expr $SLOT - 1)
export GOPATH=$1
KEYS_FOLDER=$2
CONFIGS_FOLDER=$3
TYPE=$4

echo "starting PID=$PID..."

echo "configuration:"
cat "${CONFIGS_FOLDER}/${TYPE}.json"

echo "starting gomel"
go build -gcflags='-N -l' ${GOPATH}/src/gitlab.com/alephledger/consensus-go/cmd/gomel/main.go
./main --pk ${KEYS_FOLDER}/${PID}.pk --keys_addrs ${KEYS_FOLDER}/committee.ka --log $PID.$TYPE.log --config "${CONFIGS_FOLDER}/${TYPE}.json"
wait

echo "copying logs to /extract"
mkdir /extract
cp *.log /extract

echo "exiting"
