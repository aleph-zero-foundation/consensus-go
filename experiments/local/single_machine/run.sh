#!/bin/bash

killall go
killall gomel

set -e

rm -f out

rm -f *log *pk committee.ka

go run ../../../cmd/gomel-keys 4 $1

for PID in {0..3}
do
    go run ../../../cmd/gomel --priv $PID.pk --keys_addrs committee.ka&
done

