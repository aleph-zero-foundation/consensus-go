package main

import (
	"flag"
	"fmt"
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process/run"
)

func generateKeys(nProcesses uint64) (pubKeys []gomel.PublicKey, privKeys []gomel.PrivateKey) {
	pubKeys = make([]gomel.PublicKey, 0, nProcesses)
	privKeys = make([]gomel.PrivateKey, 0, nProcesses)
	for i := uint64(0); i < nProcesses; i++ {
		pubKey, privKey, _ := signing.GenerateKeys()
		pubKeys = append(pubKeys, pubKey)
		privKeys = append(privKeys, privKey)
	}
	return pubKeys, privKeys
}

func generateLocalhostAdresses(localhostAddress string, nProcesses uint64) []string {
	const (
		magicPort = 21037
	)
	result := make([]string, 0, nProcesses)
	for id := uint64(0); id < nProcesses; id++ {
		result = append(result, fmt.Sprintf("%s:%d", localhostAddress, magicPort+id))
	}
	return result
}

func createAndStartProcess(
	id int,
	addresses []string,
	pubKeys []gomel.PublicKey,
	privKey gomel.PrivateKey,
	userDB string,
	maxLevel,
	maxHeight uint64,
	finished *sync.WaitGroup,
) error {
	committee := config.Committee{
		Pid:        id,
		PrivateKey: privKey,
		PublicKeys: pubKeys,
		Addresses:  addresses,
	}
	defaultAppConfig := config.NewDefaultConfiguration()
	config := defaultAppConfig.GenerateConfig(&committee, userDB)
	// TODO types
	// set stop condition for a process
	config.Create.MaxLevel = int(maxLevel)
	config.Create.MaxHeight = int(maxHeight)
	log, err := logging.NewLogger("stdout", 0, 100000, false)
	if err != nil {
		return err
	}
	log = log.With().Int("process_id", id).Logger()

	go func() {
		_, err := run.Process(config, log)
		if err != nil {
			log.Err(err).Msg("failed to initialize a process")
		}
		finished.Done()
	}()
	return nil
}

func main() {
	testSize := flag.Uint64("test_size", 9, "number of created processes; default is 9")
	userDB := flag.String("user_db", "../../pkg/testdata/users.txt",
		"file containing testdata for user accounts; default is a file containing names of superheros")
	maxLevel := flag.Uint64("max_level", 5, "number of levels after which a process should finish; default is 5")
	maxHeight := flag.Uint64("max_height", 5, "maximal height after which a process should finish; default is 5")
	flag.Parse()

	addresses := generateLocalhostAdresses("localhost", *testSize)
	pubKeys, privKeys := generateKeys(*testSize)

	var allDone sync.WaitGroup
	for id := range addresses {
		allDone.Add(1)
		err := createAndStartProcess(id, addresses, pubKeys, privKeys[id], *userDB, *maxLevel, *maxHeight, &allDone)
		if err != nil {
			panic(err)
		}
	}

	// wait for all processes to finish
	allDone.Wait()
	// TODO add some poset verification after all processes finished
}
