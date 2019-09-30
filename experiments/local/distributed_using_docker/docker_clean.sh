#!/bin/bash

HOSTNAME=$(hostname -i)
for i in {1..8}
do
    NODE_NAME="node${i}"
    docker service rm ${NODE_NAME}
done
wait
