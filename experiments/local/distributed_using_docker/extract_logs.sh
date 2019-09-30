#!/bin/bash

mkdir logs
for CONTAINER_ID in "$@"
do
    docker cp ${CONTAINER_ID}:/extract logs/${CONTAINER_ID}
done
