#!/bin/bash
set -e

go run ../../cmd/gomel-keys 16 addrs/$1.addrs

for PID in {0..15}
do
    go run ../../cmd/gomel --pk $PID.pk --keys_addrs committee.ka --log $PID.$1.log --config "confs/$1.json"&
done

