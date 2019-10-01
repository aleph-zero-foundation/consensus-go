#!/bin/bash

NODES=$(expr ${1:-4})
HOSTNAME=$(hostname -i)

for i in $(seq 1 $NODES)
do
    NODE_NAME="node${i}"
    docker service rm ${NODE_NAME}
done
wait
