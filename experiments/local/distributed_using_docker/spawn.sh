#!/bin/bash

NODES=$(expr ${1:-8})
HOSTNAME=$(hostname -i)

for i in $(seq 1 $NODES)
do
    NODE_NAME="node${i}"
    docker service create --name $NODE_NAME --hostname $NODE_NAME --network aleph_net -e SLOT=$i --restart-condition none ${HOSTNAME}:5000/aleph:test /work/run.sh /work/code /work/keys /work/configs config &
done
wait
