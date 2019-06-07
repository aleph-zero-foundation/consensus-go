[![Build Status](https://gitlab.com/alephledger/consensus-go/badges/devel/build.svg)](https://gitlab.com/alephledger/consensus-go/commits/devel)

# consensus-go

The main implementation of the aleph's consensus protocol in go.


# Using the Profiler
Running gomel with "--cpuprof filename" and/or "--memprof filename" flags will result in generating cpu and/or memory profile files. They can be then analyzed by running the pprof tool as follows

  - For a cpu profile file profcp
```sh
$ go tool pprof profcp
```
  - For a memory profile file profmp
```sh
$ go tool pprof --alloc_space profmp
```
The most useful commands when the tool is running are:
| Command | Result |
| ------ | ------ |
| top | shows the "heaviest" functions w.r.t. cpu/memory |
| web | opens in a web browser a nice call-graph with cpu time or memory allocation data |