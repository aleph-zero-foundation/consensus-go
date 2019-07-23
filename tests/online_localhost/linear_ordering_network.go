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
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
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
	maxLevel uint64,
	finished *sync.WaitGroup,
	dags []gomel.Dag,
) error {
	committee := config.Committee{
		Pid:        id,
		PrivateKey: privKey,
		PublicKeys: pubKeys,
		Addresses:  addresses,
	}
	defaultAppConfig := config.NewDefaultConfiguration()
	config := defaultAppConfig.GenerateConfig(&committee, userDB)
	// set stop condition for a process
	config.Create.MaxLevel = int(maxLevel)
	log, err := logging.NewLogger("log"+strconv.Itoa(id)+".log", 0, 100000, false)
	if err != nil {
		return err
	}
	log = log.With().Int("process_id", id).Logger()

	go func() {
		dag, err := run.Process(config, log)
		if err != nil {
			log.Err(err).Msg("failed to initialize a process")
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
			if nDagsContainingUnit[*u.Hash()] != dag.NProc() {
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

func checkOrderingFromLogs(nProc int) bool {
	var lastOrder [][2]int
	for pid := 0; pid < nProc; pid++ {
		myOrder := readOrderFromLogs("log" + strconv.Itoa(pid) + ".log")
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
	testSize := flag.Uint64("test_size", 10, "number of created processes; default is 10")
	userDB := flag.String("user_db", "../../pkg/testdata/users.txt",
		"file containing testdata for user accounts; default is a file containing names of superheros")
	maxLevel := flag.Uint64("max_level", 10, "number of levels after which a process should finish; default is 10")
	flag.Parse()

	addresses := generateLocalhostAdresses("localhost", *testSize)
	pubKeys, privKeys := generateKeys(*testSize)
	dags := make([]gomel.Dag, int(*testSize))

	var allDone sync.WaitGroup
	for id := range addresses {
		allDone.Add(1)
		err := createAndStartProcess(id, addresses, pubKeys, privKeys[id], *userDB, *maxLevel, &allDone, dags)
		if err != nil {
			panic(err)
		}
	}

	// wait for all processes to finish
	allDone.Wait()

	// Sanity checks
	fmt.Println("Dags are the same up to", commonLevel(dags, int(*maxLevel)), "level. Max level is", *maxLevel)
	if checkOrderingFromLogs(dags[0].NProc()) {
		fmt.Println("Ordering OK")
	} else {
		fmt.Println("Processes obtained different ordering!")
	}
}
