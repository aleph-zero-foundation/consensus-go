#!/bin/bash
set -e

go run ../../cmd/gomel-keys 4 $1_addrs

for PID in {0..3}
do
    go run ../../cmd/gomel --pk $PID.pk --keys_addrs keys_addrs --log $PID.log --config "confs/$1.json"&
done

