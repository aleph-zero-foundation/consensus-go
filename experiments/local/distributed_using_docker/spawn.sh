#!/bin/bash

for i in {1..4}
do
    NODE_NAME="node_${i}"
    docker service create --name $NODE_NAME --hostname $NODE_NAME --network aleph_net -e SLOT=$i localhost:5000/aleph:test /work/run.sh /work/code /work/keys /work/configs g &
done
wait
