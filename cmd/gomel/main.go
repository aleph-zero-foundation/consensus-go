package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/process/run"
)

func generatePosetConfig(publicKeys []gomel.PublicKey) *gomel.PosetConfig {
	return &gomel.PosetConfig{
		Keys: publicKeys,
	}
}

func generateSyncConfig(conf *config.Configuration, id int, remoteAddresses []string, address string) *process.Sync {
	return &process.Sync{
		Pid:                  id,
		LocalAddress:         address,
		RemoteAddresses:      remoteAddresses,
		ListenQueueLength:    conf.NRecvSync,
		SyncQueueLength:      conf.NInitSync,
		InitializedSyncLimit: conf.NInitSync,
		ReceivedSyncLimit:    conf.NRecvSync,
		SyncInitDelay:        time.Duration(conf.SyncInitDelay * float32(time.Second)),
	}
}

func generateCreateConfig(conf *config.Configuration, id int, privateKey gomel.PrivateKey) *process.Create {
	// TODO: magic number
	maxHeight := 2137
	if conf.UnitsLimit != nil {
		maxHeight = int(*conf.UnitsLimit)
	}
	// TODO: magic number in adjust factor
	return &process.Create{
		Pid:          id,
		MaxParents:   int(conf.NParents),
		PrivateKey:   privateKey,
		InitialDelay: time.Duration(conf.CreateDelay * float32(time.Second)),
		AdjustFactor: 0.14,
		MaxLevel:     int(conf.LevelLimit),
		MaxHeight:    maxHeight,
	}
}

func generateOrderConfig(conf *config.Configuration) *process.Order {
	return &process.Order{
		VotingLevel:  int(conf.VotingLevel),
		PiDeltaLevel: int(conf.PiDeltaLevel),
	}
}

func generateTxValidateConfig(dbFilename string) *process.TxValidate {
	return &process.TxValidate{
		UserDb: dbFilename,
	}
}

func generateTxGenerateConfig(dbFilename string) *process.TxGenerate {
	return &process.TxGenerate{
		UserDb: dbFilename,
	}
}

func generateConfig(conf *config.Configuration, publicKeys []gomel.PublicKey, remoteAddresses []string, privateKey gomel.PrivateKey, address, dbFilename string) process.Config {
	id := 0
	for i, a := range remoteAddresses {
		if address == a {
			id = i
			break
		}
	}
	return process.Config{
		Poset:      generatePosetConfig(publicKeys),
		Sync:       generateSyncConfig(conf, id, remoteAddresses, address),
		Create:     generateCreateConfig(conf, id, privateKey),
		Order:      generateOrderConfig(conf),
		TxValidate: generateTxValidateConfig(dbFilename),
		TxGenerate: generateTxGenerateConfig(dbFilename),
	}
}

func getKeys(filename string) ([]gomel.PublicKey, []string, gomel.PrivateKey, string, error) {
	if filename == "" {
		return nil, nil, nil, "", errors.New("please provide a key file")
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, nil, "", err
	}
	defer file.Close()
	var publicKeys []gomel.PublicKey
	var remoteAddresses []string
	var privateKey gomel.PrivateKey
	var address string
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanWords)
	if !scanner.Scan() {
		return nil, nil, nil, "", errors.New("key file too short")
	}
	privateKey, err = signing.DecodePrivateKey(scanner.Text())
	if err != nil {
		return nil, nil, nil, "", err
	}
	if !scanner.Scan() {
		return nil, nil, nil, "", errors.New("key file too short")
	}
	address = scanner.Text()
	for scanner.Scan() {
		publicKey, err := signing.DecodePublicKey(scanner.Text())
		if err != nil {
			return nil, nil, nil, "", err
		}
		publicKeys = append(publicKeys, publicKey)
		if !scanner.Scan() {
			return nil, nil, nil, "", errors.New("key file too short")
		}
		remoteAddresses = append(remoteAddresses, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, nil, "", err
	}
	if len(publicKeys) < 4 {
		return nil, nil, nil, "", errors.New("key file too short")
	}
	return publicKeys, remoteAddresses, privateKey, address, nil
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

func getOptions() (string, string, string) {
	var keyFilename, configFilename, dbFilename string
	flag.StringVar(&keyFilename, "keys", "", "a file with keys and associated addresses")
	flag.StringVar(&configFilename, "config", "", "a configuration file")
	flag.StringVar(&dbFilename, "db", "", "a mock database file")
	flag.Parse()
	return keyFilename, configFilename, dbFilename
}

func main() {
	keyFilename, configFilename, dbFilename := getOptions()
	publicKeys, remoteAddresses, privateKey, address, err := getKeys(keyFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid key file \"%s\", because: %s.\n", keyFilename, err.Error())
		return
	}
	conf, err := getConfiguration(configFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration file \"%s\", because: %s.\n", configFilename, err.Error())
		return
	}
	processConfig := generateConfig(conf, publicKeys, remoteAddresses, privateKey, address, dbFilename)
	err = run.Process(processConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Process died with %s.\n", err.Error())
	}
}
