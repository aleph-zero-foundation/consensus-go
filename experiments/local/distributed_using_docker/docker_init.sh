#!/bin/bash
set -e

echo "initializing docker in swarm-mode"
docker swarm init

echo "creating overlay network"
docker network create -d overlay --attachable aleph_net

echo "cloning consensus-go repository into `code`"
mkdir -p code/src/gitlab.com/alephledger
git clone git@gitlab.com:alephledger/consensus-go.git code/src/gitlab.com/alephledger/consensus-go

echo "building docker image"
HOSTNAME=$(hostname -i)
docker build -t ${HOSTNAME}:5000/aleph:test .

echo "pushing docker image into local repository"
docker service create --name registry --publish published=5000,target=5000 registry:2
docker push ${HOSTNAME}:5000/aleph:test
