package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/encrypt"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/run"
)

func generateKeys(nProcesses uint16) (pubKeys []gomel.PublicKey, privKeys []gomel.PrivateKey) {
	pubKeys = make([]gomel.PublicKey, 0, nProcesses)
	privKeys = make([]gomel.PrivateKey, 0, nProcesses)
	for i := uint16(0); i < nProcesses; i++ {
		pubKey, privKey, _ := signing.GenerateKeys()
		pubKeys = append(pubKeys, pubKey)
		privKeys = append(privKeys, privKey)
	}
	return pubKeys, privKeys
}

func generateRMCKeys(nProcesses uint16) (sekKeys []*bn256.SecretKey, verKeys []*bn256.VerificationKey) {
	sekKeys = make([]*bn256.SecretKey, 0, nProcesses)
	verKeys = make([]*bn256.VerificationKey, 0, nProcesses)
	for i := uint16(0); i < nProcesses; i++ {
		verKey, sekKey, _ := bn256.GenerateKeys()
		sekKeys = append(sekKeys, sekKey)
		verKeys = append(verKeys, verKey)
	}
	return
}

func generateEncKeys(nProcesses uint16) (encKeys []encrypt.EncryptionKey, decKeys []encrypt.DecryptionKey) {
	encKeys = make([]encrypt.EncryptionKey, 0, nProcesses)
	decKeys = make([]encrypt.DecryptionKey, 0, nProcesses)
	for i := uint16(0); i < nProcesses; i++ {
		encKey, decKey, _ := encrypt.GenerateKeys()
		encKeys = append(encKeys, encKey)
		decKeys = append(decKeys, decKey)
	}
	return
}

func generateLocalhostAddresses(localhostAddress string, nProcesses int) ([]string, []string, []string, []string) {
	const (
		magicPort = 21037
	)
	result := make([]string, 0, nProcesses)
	resultMC := make([]string, 0, nProcesses)
	setupResult := make([]string, 0, nProcesses)
	setupMCResult := make([]string, 0, nProcesses)
	for id := 0; id < nProcesses; id++ {
		result = append(result, fmt.Sprintf("%s:%d", localhostAddress, magicPort+4*id))
		resultMC = append(resultMC, fmt.Sprintf("%s:%d", localhostAddress, magicPort+4*id+1))
		setupResult = append(setupResult, fmt.Sprintf("%s:%d", localhostAddress, magicPort+4*id+2))
		setupMCResult = append(setupMCResult, fmt.Sprintf("%s:%d", localhostAddress, magicPort+4*id+3))
	}
	return result, setupResult, resultMC, setupMCResult
}

func createAndStartProcess(
	id uint16,
	addresses []string,
	setupAddresses []string,
	mcAddresses []string,
	setupMCAddresses []string,
	pubKeys []gomel.PublicKey,
	privKey gomel.PrivateKey,
	verificationKeys []*bn256.VerificationKey,
	secretKey *bn256.SecretKey,
	eKeys []encrypt.EncryptionKey,
	dKey encrypt.DecryptionKey,
	userDB string,
	maxLevel int,
	finished *sync.WaitGroup,
	dags []gomel.Dag,
) error {
	member := config.Member{
		Pid:          id,
		PrivateKey:   privKey,
		RMCSecretKey: secretKey,
		DKey:         dKey,
	}
	committee := config.Committee{
		PublicKeys:          pubKeys,
		RMCVerificationKeys: verificationKeys,
		EKeys:               eKeys,
		SetupAddresses:      [][]string{setupAddresses, setupMCAddresses},
		Addresses:           [][]string{addresses, mcAddresses},
	}
	defaultAppConfig := config.NewDefaultConfiguration()
	defaultAppConfig.OrderStartLevel = 6
	config := defaultAppConfig.GenerateConfig(&member, &committee)
	// set stop condition for a process
	config.Create.MaxLevel = int(maxLevel)

	setupLog, err := logging.NewLogger("setup_log"+strconv.Itoa(int(id))+".log", 0, 100000, false)
	if err != nil {
		return err
	}

	log, err := logging.NewLogger("log"+strconv.Itoa(int(id))+".log", 0, 100000, false)
	if err != nil {
		return err
	}

	go func() {
		dag, err := run.Process(config, setupLog, log)
		if err != nil {
			log.Err(err).Msg("failed to initialize a process")
			panic(err)
		}
		dags[id] = dag
		finished.Done()
	}()
	return nil
}

func collectUnits(dag gomel.Dag) map[gomel.Unit]bool {
	seenUnits := make(map[gomel.Unit]bool)
	var dfs func(u gomel.Unit)
	dfs = func(u gomel.Unit) {
		seenUnits[u] = true
		for _, uParent := range u.Parents() {
			if uParent == nil {
				continue
			}
			if !seenUnits[uParent] {
				dfs(uParent)
			}
		}
	}
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			if !seenUnits[u] {
				dfs(u)
			}
		}
		return true
	})
	return seenUnits
}

func commonLevel(dags []gomel.Dag, maxLevel int) int {
	// counting how many dags contains a unit
	nDagsContainingUnit := make(map[gomel.Hash]int)
	for _, dag := range dags {
		for u := range collectUnits(dag) {
			nDagsContainingUnit[*u.Hash()]++
		}
	}

	// counting how many levels the dags have in common
	commonLevel := maxLevel
	for _, dag := range dags {
		for u := range collectUnits(dag) {
			if nDagsContainingUnit[*u.Hash()] != int(dag.NProc()) {
				if u.Level() <= commonLevel {
					commonLevel = u.Level() - 1
				}
			}
		}
	}
	return commonLevel
}

func isPrefix(ord1, ord2 [][2]int) bool {
	if len(ord1) > len(ord2) {
		return false
	}
	for i, val := range ord1 {
		if val[0] != ord2[i][0] || val[1] != ord2[i][1] {
			return false
		}
	}
	return true
}

// returns slice of pairs (creator, height) of units in order
func readOrderFromLogs(logfile string) [][2]int {
	result := [][2]int{}
	file, err := os.Open(logfile)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var data map[string]interface{}
		json.Unmarshal([]byte(scanner.Text()), &data)
		if service, ok := data[logging.Service]; ok {
			if int(service.(float64)) == logging.ValidateService {
				if event, ok := data[logging.Event]; ok {
					if event.(string) == logging.DataValidated {
						result = append(result, [2]int{int(data[logging.Creator].(float64)), int(data[logging.Height].(float64))})
					}
				}
			}
		}
	}
	return result
}

func checkOrderingFromLogs(nProc uint16, filenamePrefix string) bool {
	var lastOrder [][2]int
	for pid := uint16(0); pid < nProc; pid++ {
		myOrder := readOrderFromLogs(filenamePrefix + strconv.Itoa(int(pid)) + ".log")
		if pid != 0 && !isPrefix(lastOrder, myOrder) && !isPrefix(myOrder, lastOrder) {
			return false
		}
		if len(lastOrder) < len(myOrder) {
			lastOrder = myOrder
		}
	}
	return true
}

func main() {
	testSize := flag.Int("test_size", 10, "number of created processes; default is 10")
	userDB := flag.String("user_db", "../../pkg/testdata/users.txt",
		"file containing testdata for user accounts; default is a file containing names of superheros")
	maxLevel := flag.Int("max_level", 12, "number of levels after which a process should finish; default is 12")
	flag.Parse()

	addresses, setupAddresses, mcAddresses, setupMCAddresses := generateLocalhostAddresses("localhost", *testSize)
	pubKeys, privKeys := generateKeys(uint16(*testSize))
	sekKeys, verKeys := generateRMCKeys(uint16(*testSize))
	eKeys, dKeys := generateEncKeys(uint16(*testSize))
	dags := make([]gomel.Dag, int(*testSize))

	var allDone sync.WaitGroup
	for id := range addresses {
		allDone.Add(1)
		err := createAndStartProcess(uint16(id), addresses, setupAddresses, mcAddresses, setupMCAddresses, pubKeys, privKeys[id], verKeys, sekKeys[id], eKeys, dKeys[id], *userDB, *maxLevel, &allDone, dags)
		if err != nil {
			panic(err)
		}
	}

	// wait for all processes to finish
	allDone.Wait()
	// Sanity checks
	if checkOrderingFromLogs(dags[0].NProc(), "setup_log") {
		fmt.Println("Ordering in setup OK")
	} else {
		fmt.Println("Processes obtained different orderings in setup!")
	}
	fmt.Println("Main Dags are the same up to", commonLevel(dags, int(*maxLevel)), "level. Max level is", *maxLevel)
	if checkOrderingFromLogs(dags[0].NProc(), "log") {
		fmt.Println("Ordering in main is OK")
	} else {
		fmt.Println("Processes obtained different orderings in main!")
	}
}
