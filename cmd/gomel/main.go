package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/run"
	"gitlab.com/alephledger/core-go/pkg/core"
	"gitlab.com/alephledger/core-go/pkg/tests"
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
	data              int
	epochs            int
	units             int
	output            int
	setup             bool
	mutexFraction     int
	blockFraction     int
	delay             int64
}

func getOptions() cliOptions {
	var result cliOptions
	flag.BoolVar(&result.setup, "setup", true, "a flag whether a setup should be run")
	flag.StringVar(&result.privFilename, "priv", "", "a file with private keys and process id")
	flag.StringVar(&result.keysAddrsFilename, "keys_addrs", "", "a file with keys and associated addresses")
	flag.IntVar(&result.data, "data", 0, "size [kB] of random data to be put in every unit (-1 to enable reading unit data from stdin)")
	flag.IntVar(&result.epochs, "epochs", 0, "number of epochs to run")
	flag.IntVar(&result.units, "units", 0, "number of levels to produce in each epoch")
	flag.IntVar(&result.output, "output", 2, "type of preblock consumer (0 ignore, 1 control sum, 2 data")
	flag.StringVar(&result.cpuProfFilename, "cpuprof", "", "the name of the file with cpu-profile results")
	flag.StringVar(&result.memProfFilename, "memprof", "", "the name of the file with mem-profile results")
	flag.StringVar(&result.traceFilename, "trace", "", "the name of the file with trace-profile results")
	flag.IntVar(&result.mutexFraction, "mf", 0, "the sampling fraction of mutex contention events")
	flag.IntVar(&result.blockFraction, "bf", 0, "the sampling fraction of goroutine blocking events")
	flag.Int64Var(&result.delay, "delay", 0, "number of seconds to wait before running the protocol")
	flag.Parse()
	return result
}

func main() {
	options := getOptions()

	// wait if asked to
	if options.delay != 0 {
		duration := time.Duration(options.delay) * time.Second
		time.Sleep(duration)
	}

	// set up profilers
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
	// get committee config
	consensusConfig := config.New(member, committee)
	if err := config.Valid(consensusConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid consensus configuration because: %s.\n", err.Error())
		return
	}
	if options.epochs != 0 {
		consensusConfig.NumberOfEpochs = options.epochs
	}
	if options.units != 0 {
		consensusConfig.EpochLength = options.units
		consensusConfig.LastLevel = consensusConfig.EpochLength + consensusConfig.OrderStartLevel - 1
	}

	// create mock data source
	var dataSource core.DataSource
	if options.data == -1 {
		dataSource = tests.StdinDataSource()
	} else {
		dataSource = tests.RandomDataSource(300 * options.data)
	}

	// create preblock sink with mock consumer
	preblockSink := make(chan *core.Preblock)
	done := make(chan struct{})
	go func() {
		defer close(done)
		switch options.output {
		case 1:
			tests.ControlSumPreblockConsumer(preblockSink, os.Stdout)
		case 2:
			tests.DataExtractingPreblockConsumer(preblockSink, os.Stdout)
		default:
			tests.NopPreblockConsumer(preblockSink)
		}
	}()

	// initialize process
	var start, stop func()
	if options.setup {
		setupConfig := config.NewSetup(member, committee)
		if err := config.ValidSetup(setupConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid setup configuration because: %s.\n", err.Error())
			return
		}
		start, stop, err = run.Process(setupConfig, consensusConfig, dataSource, preblockSink)
	} else {
		start, stop, err = run.NoBeacon(consensusConfig, dataSource, preblockSink)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Process died with %s.\n", err.Error())
		return
	}

	// run process
	start()
	<-done
	stop()

	// dump profiles
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

	// give logger a chance to finish
	time.Sleep(time.Second)
	fmt.Fprintf(os.Stdout, "All done!\n")
}
