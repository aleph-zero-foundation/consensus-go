#!/bin/bash

for i in {1..8}
do
    NODE_NAME="node${i}"
    docker service create --name $NODE_NAME --hostname $NODE_NAME --network aleph_net -e SLOT=$i localhost:5000/aleph:test /work/run.sh /work/code /work/keys /work/configs config &
done
wait
