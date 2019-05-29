package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process/run"
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
	keyFilename    string
	configFilename string
	dbFilename     string
	logFilename    string
}

func getOptions() cliOptions {
	var result cliOptions
	flag.StringVar(&result.keyFilename, "keys", "", "a file with keys and associated addresses")
	flag.StringVar(&result.configFilename, "config", "", "a configuration file")
	flag.StringVar(&result.dbFilename, "db", "", "a mock database file")
	flag.StringVar(&result.logFilename, "log", "", "the name of the file with logs")
	flag.Parse()
	return result
}

func main() {
	options := getOptions()
	if options.logFilename == "" {
		options.logFilename = "aleph.log"
	}
	logging.InitLogger(logging.LogConfig{
		Level:    1,
		Path:     options.logFilename,
		DiodeBuf: 100000,
		TimeUnit: time.Millisecond,
	})
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
	processConfig := conf.GenerateConfig(committee, options.dbFilename)
	err = run.Process(processConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Process died with %s.\n", err.Error())
	}
}
