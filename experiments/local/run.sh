#!/bin/bash

go run ../../cmd/gomel-keys 4

for PID in {0..3}
do
    go run ../../cmd/gomel --keys $PID.keys --log $PID.log --config config.json --db ../../pkg/testdata/users.txt&
done
