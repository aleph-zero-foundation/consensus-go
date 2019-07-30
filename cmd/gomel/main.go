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
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/process/run"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

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
	keyFilename     string
	configFilename  string
	logFilename     string
	cpuProfFilename string
	memProfFilename string
	traceFilename   string
	dagFilename     string
	localAddress    string
	localMcAddress  string
	delay           int64
}

func getOptions() cliOptions {
	var result cliOptions
	flag.StringVar(&result.keyFilename, "keys", "", "a file with keys and associated addresses")
	flag.StringVar(&result.configFilename, "config", "", "a configuration file")
	flag.StringVar(&result.logFilename, "log", "aleph.log", "the name of the file with logs")
	flag.StringVar(&result.cpuProfFilename, "cpuprof", "", "the name of the file with cpu-profile results")
	flag.StringVar(&result.memProfFilename, "memprof", "", "the name of the file with mem-profile results")
	flag.StringVar(&result.traceFilename, "trace", "", "the name of the file with trace-profile results")
	flag.StringVar(&result.dagFilename, "dag", "", "the name of the file to save resulting dag")
	flag.StringVar(&result.localAddress, "address", "", "the gossip address on which to run the process, if omitted will be read from the key file")
	flag.StringVar(&result.localMcAddress, "mcAddress", "", "the MC address on which to run the process, if omitted will be read from the key file")
	flag.Int64Var(&result.delay, "delay", 0, "number of seconds to wait before running the protocol")
	flag.Parse()
	return result
}

func fixLocalAddress(processConfig process.Config, localAddress string) {
	if localAddress != "" {
		processConfig.Sync.LocalAddress = localAddress
	}
}

func fixLocalMcAddress(processConfig process.Config, localMcAddress string) {
	if localMcAddress != "" {
		processConfig.Sync.LocalMCAddress = localMcAddress
	}
}

func main() {
	options := getOptions()

	if options.delay != 0 {
		duration := time.Duration(options.delay) * time.Second
		time.Sleep(duration)
	}

	committee, err := getCommittee(options.keyFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid key file \"%s\", because: %s.\n", options.keyFilename, err.Error())
		return
	}
	conf, err := getConfiguration(options.configFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration file \"%s\", because: %s.\n", options.configFilename, err.Error())
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

	processConfig := conf.GenerateConfig(committee)

	fixLocalAddress(processConfig, options.localAddress)
	fixLocalMcAddress(processConfig, options.localMcAddress)

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

	var dag gomel.Dag
	dag, err = run.Process(processConfig, setupLog, log, run.UrnSetup)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Process died with %s.\n", err.Error())
	}

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
