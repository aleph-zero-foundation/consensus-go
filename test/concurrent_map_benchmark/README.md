# Concurrent map benchmark

Runs a benchmark for several possible implementations of a thread-safe map type. It uses some preconfigured number of manually
spawned goroutines. It tries to simulate an access pattern that resembles expected behavior of aleph's implementation.

## Usage

In order to run this benchmark execute in shell:
```shell
go test -bench=. -benchtime=20s -cpuprofile profile.out
```

## pprof

To diagnose performance bottlenecks execute in shell:
```shell
go tool pprof profile.out
```

Inside of the spawned shell you can execute following:
```
(pprof) top
(pprof) list <bottleneck_execution_point>
```
