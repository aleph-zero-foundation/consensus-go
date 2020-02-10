package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sync"
	"syscall"
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/run"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
	"gitlab.com/alephledger/core-go/pkg/core"
)

func getMember(filename string) (*config.Member, error) {
	if filename == "" {
		return nil, errors.New("please provide a key file")
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
		return nil, errors.New("please provide a key file")
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return config.LoadCommittee(file)
}

func getConfiguration(filename string) (*config.Params, error) {
	var result config.Params
	if filename == "" {
		result = config.NewDefaultConfiguration()
		return &result, nil
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	err = config.NewJSONConfigLoader().LoadConfiguration(file, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

type cliOptions struct {
	pkPidFilename     string
	keysAddrsFilename string
	configFilename    string
	logFilename       string
	cpuProfFilename   string
	memProfFilename   string
	traceFilename     string
	dagFilename       string
	mutexFraction     int
	blockFraction     int
	delay             int64
}

func getOptions() cliOptions {
	var result cliOptions
	flag.StringVar(&result.pkPidFilename, "pk", "", "a file with a private key and process id")
	flag.StringVar(&result.keysAddrsFilename, "keys_addrs", "", "a file with keys and associated addresses")
	flag.StringVar(&result.configFilename, "config", "", "a configuration file")
	flag.StringVar(&result.logFilename, "log", "aleph.log", "the name of the file with logs")
	flag.StringVar(&result.cpuProfFilename, "cpuprof", "", "the name of the file with cpu-profile results")
	flag.StringVar(&result.memProfFilename, "memprof", "", "the name of the file with mem-profile results")
	flag.StringVar(&result.traceFilename, "trace", "", "the name of the file with trace-profile results")
	flag.StringVar(&result.dagFilename, "dag", "", "the name of the file to save resulting dag")
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

	member, err := getMember(options.pkPidFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid private key file \"%s\", because: %s.\n", options.pkPidFilename, err.Error())
		return
	}
	committee, err := getCommittee(options.keysAddrsFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid key file \"%s\", because: %s.\n", options.keysAddrsFilename, err.Error())
		return
	}
	conf, err := getConfiguration(options.configFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration file \"%s\", because: %s.\n", options.configFilename, err.Error())
		return
	}
	if len(conf.SyncSetup) != len(committee.SetupAddresses) {
		fmt.Fprintf(os.Stderr, "Wrong number of setup addresses. Needs %d, got %d", len(conf.SyncSetup), len(committee.SetupAddresses))
		return
	}
	// The additional address is for alerts.
	if len(conf.Sync)+1 != len(committee.Addresses) {
		fmt.Println(committee.Addresses)
		fmt.Fprintf(os.Stderr, "Wrong number of addresses. Needs %d, got %d", len(conf.Sync)+1, len(committee.Addresses))
		return
	}

	log, err := logging.NewLogger(options.logFilename, conf.LogLevel, conf.LogBuffer, conf.LogHuman)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Creating log file \"%s\" failed because: %s.\n", options.logFilename, err.Error())
		return
	}
	setupLog, err := logging.NewLogger("setup_"+options.logFilename, conf.LogLevel, conf.LogBuffer, conf.LogHuman)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Creating log file \"%s\" failed because: %s.\n", "setup_"+options.logFilename, err.Error())
		return
	}

	processConfig := conf.GenerateConfig(member, committee)

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

	// Mock data source and preblock sink.
	tds := tests.NewDataSource(300 * conf.Txpu)
	tds.Start()
	ps := make(chan *core.Preblock)
	var wait sync.WaitGroup
	wait.Add(1)
	// Reading and ignoring all the preblocks.
	go func() {
		defer wait.Done()
		for range ps {
		}
	}()

	fmt.Fprintln(os.Stdout, "Starting process...")

	setupErrors := make(chan error)
	mainServiceErrors := make(chan error)
	wait.Add(1)
	go func() {
		defer wait.Done()
		for err := range setupErrors {
			panic("error in setup: " + err.Error())
		}
	}()
	wait.Add(1)
	go func() {
		defer wait.Done()
		for err := range mainServiceErrors {
			panic("error in main service: " + err.Error())
		}
	}()

	createdDag := make(chan gomel.Dag)

	dagService, err := run.Process(processConfig, tds.DataSource(), ps, createdDag, setupLog, log, setupErrors, mainServiceErrors)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Process died with %s.\n", err.Error())
	}
	err = dagService.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Main service died with %s.\n", err.Error())
	}
	dag := <-createdDag
	dagService.Stop()
	close(setupErrors)
	close(mainServiceErrors)

	tds.Stop()
	close(ps)
	wait.Wait()

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

	if options.dagFilename != "" {
		f, err := os.Create(options.dagFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Creating dag file \"%s\" failed because: %s.\n", options.dagFilename, err.Error())
		}
		defer f.Close()
		out := bufio.NewWriter(f)
		tests.WriteDag(out, dag)
		out.Flush()
	}

	fmt.Fprintf(os.Stdout, "All done! :)\n")
}
