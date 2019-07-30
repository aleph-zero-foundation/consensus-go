#!/bin/bash
set -e

go run ../../cmd/gomel-keys 4 addrs/$1.addrs

for PID in {0..3}
do
    go run ../../cmd/gomel --pk $PID.pk --keys_addrs keys_addrs --log $PID.$1.log --config "confs/$1.json"&
done

