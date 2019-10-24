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
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/run"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
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

func getConfiguration(filename string) (*config.Configuration, error) {
	var result config.Configuration
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
	flag.Int64Var(&result.delay, "delay", 0, "number of seconds to wait before running the protocol")
	flag.Parse()
	return result
}

func main() {
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

	tds := tests.NewDataSource(1000)
	tds.Start()
	var dag gomel.Dag
	dag, err = run.Process(processConfig, tds.DataSource(), setupLog, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Process died with %s.\n", err.Error())
	}
	tds.Stop()

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
}
