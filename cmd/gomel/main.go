package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"syscall"
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/run"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
	"gitlab.com/alephledger/core-go/pkg/core"
)

func getMember(filename string) (*config.Member, error) {
	if filename == "" {
		return nil, errors.New("please provide a file with private keys and pid")
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return config.LoadMember(file)
}

func getCommittee(filename string) (*config.Committee, error) {
	if filename == "" {
		return nil, errors.New("please provide a file with keys and addresses of the committee")
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return config.LoadCommittee(file)
}

type cliOptions struct {
	privFilename      string
	keysAddrsFilename string
	cpuProfFilename   string
	memProfFilename   string
	traceFilename     string
	setup             bool
	txpu              int
	mutexFraction     int
	blockFraction     int
	delay             int64
}

func getOptions() cliOptions {
	var result cliOptions
	flag.BoolVar(&result.setup, "setup", true, "a flag whether a setup should be run")
	flag.StringVar(&result.privFilename, "priv", "", "a file with private keys and process id")
	flag.StringVar(&result.keysAddrsFilename, "keys_addrs", "", "a file with keys and associated addresses")
	flag.StringVar(&result.cpuProfFilename, "cpuprof", "", "the name of the file with cpu-profile results")
	flag.StringVar(&result.memProfFilename, "memprof", "", "the name of the file with mem-profile results")
	flag.StringVar(&result.traceFilename, "trace", "", "the name of the file with trace-profile results")
	flag.IntVar(&result.txpu, "txpu", 0, "number of transactions to put to every unit")
	flag.IntVar(&result.mutexFraction, "mf", 0, "the sampling fraction of mutex contention events")
	flag.IntVar(&result.blockFraction, "bf", 0, "the sampling fraction of goroutine blocking events")
	flag.Int64Var(&result.delay, "delay", 0, "number of seconds to wait before running the protocol")
	flag.Parse()
	return result
}

func main() {
	// temporary trick to capture stdout and stderr on remote instances
	logFile, _ := os.OpenFile("out", os.O_WRONLY|os.O_CREATE|os.O_SYNC, 0644)
	syscall.Dup2(int(logFile.Fd()), 1)
	syscall.Dup2(int(logFile.Fd()), 2)

	options := getOptions()

	if options.delay != 0 {
		duration := time.Duration(options.delay) * time.Second
		time.Sleep(duration)
	}

	// get member
	member, err := getMember(options.privFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid private key file \"%s\", because: %s.\n", options.privFilename, err.Error())
		return
	}
	// get committee
	committee, err := getCommittee(options.keysAddrsFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid key file \"%s\", because: %s.\n", options.keysAddrsFilename, err.Error())
		return
	}
	// get setup config
	setupConfig := config.NewSetup(member, committee)
	if err := config.Valid(setupConfig, true); options.setup && err != nil {
		fmt.Fprintf(os.Stderr, "Invalid setup configuration because: %s.\n", err.Error())
		return
	}
	// get process config
	consensusConfig := config.New(member, committee)
	if err := config.Valid(consensusConfig, false); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid consensus configuration because: %s.\n", err.Error())
		return
	}

	if options.cpuProfFilename != "" {
		f, err := os.Create(options.cpuProfFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Creating cpu-profile file \"%s\" failed because: %s.\n", options.cpuProfFilename, err.Error())
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Cpu-profile failed to start because: %s", err.Error())
		}
		defer pprof.StopCPUProfile()
		runtime.SetMutexProfileFraction(options.mutexFraction)
		runtime.SetBlockProfileRate(options.blockFraction)
	}
	if options.traceFilename != "" {
		f, err := os.Create(options.traceFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Creating trace-profile file \"%s\" failed because: %s.\n", options.traceFilename, err.Error())
		}
		defer f.Close()
		trace.Start(f)
		defer trace.Stop()
	}

	fmt.Fprintln(os.Stdout, "Starting process...")

	// Mock data source and preblock sink.
	tds := tests.NewDataSource(300 * options.txpu)
	ps := make(chan *core.Preblock)

	var start, stop func()
	if len(setupConfig.RMCAddresses) == 0 {
		start, stop, err = run.NoBeacon(consensusConfig, tds, ps)
	} else {
		start, stop, err = run.Process(setupConfig, consensusConfig, tds, ps)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Process died with %s.\n", err.Error())
		return
	}
	start()
	seenPB, expectedPB := 0, consensusConfig.EpochLength*consensusConfig.NumberOfEpochs
	for range ps {
		seenPB++
		if seenPB == expectedPB {
			break
		}
	}

	close(ps)
	stop()

	if options.memProfFilename != "" {
		f, err := os.Create(options.memProfFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Creating mem-profile file \"%s\" failed because: %s.\n", options.memProfFilename, err.Error())
		}
		defer f.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Mem-profile failed to start because: %s", err.Error())
		}
	}

	fmt.Fprintf(os.Stdout, "All done!\n")
}
