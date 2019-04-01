# Concurrent map benchmark

## Usage

```shell
go test -bench=. -benchtime=20s -cpuprofile profile.out
go tool pprof profile.out
top
```

## pprof

```shell
go tool pprof profile.out
top
```
