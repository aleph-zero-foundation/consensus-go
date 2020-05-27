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
	"gitlab.com/alephledger/consensus-go/pkg/run"
	"gitlab.com/alephledger/core-go/pkg/core"
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/core-go/pkg/crypto/p2p"
	"gitlab.com/alephledger/core-go/pkg/tests"
)

func generateKeys(nProcesses uint16) (pubKeys []gomel.PublicKey, privKeys []gomel.PrivateKey) {
	pubKeys = make([]gomel.PublicKey, nProcesses)
	privKeys = make([]gomel.PrivateKey, nProcesses)
	for i := uint16(0); i < nProcesses; i++ {
		pubKeys[i], privKeys[i], _ = signing.GenerateKeys()
	}
	return pubKeys, privKeys
}

func generateRMCKeys(nProcesses uint16) (sekKeys []*bn256.SecretKey, verKeys []*bn256.VerificationKey) {
	sekKeys = make([]*bn256.SecretKey, nProcesses)
	verKeys = make([]*bn256.VerificationKey, nProcesses)
	for i := uint16(0); i < nProcesses; i++ {
		verKeys[i], sekKeys[i], _ = bn256.GenerateKeys()
	}
	return
}

func generateP2PKeys(nProcesses uint16) (p2pPubKeys []*p2p.PublicKey, p2pSecKeys []*p2p.SecretKey) {
	p2pPubKeys = make([]*p2p.PublicKey, nProcesses)
	p2pSecKeys = make([]*p2p.SecretKey, nProcesses)
	for i := uint16(0); i < nProcesses; i++ {
		p2pPubKeys[i], p2pSecKeys[i], _ = p2p.GenerateKeys()
	}
	return
}

func generateLocalhostAddresses(localhostAddress string, nProcesses int) (gossip []string, mc []string, fetch []string, rmc []string, setupGossip []string, setupMC []string, setupFetch []string, setupRMC []string) {
	const (
		magicPort = 21037
	)
	gossip = make([]string, nProcesses)
	mc = make([]string, nProcesses)
	fetch = make([]string, nProcesses)
	rmc = make([]string, nProcesses)

	setupGossip = make([]string, nProcesses)
	setupMC = make([]string, nProcesses)
	setupFetch = make([]string, nProcesses)
	setupRMC = make([]string, nProcesses)

	for id := 0; id < nProcesses; id++ {
		gossip[id] = fmt.Sprintf("%s:%d", localhostAddress, magicPort+8*id)
		mc[id] = fmt.Sprintf("%s:%d", localhostAddress, magicPort+8*id+1)
		fetch[id] = fmt.Sprintf("%s:%d", localhostAddress, magicPort+8*id+2)
		rmc[id] = fmt.Sprintf("%s:%d", localhostAddress, magicPort+8*id+3)

		setupGossip[id] = fmt.Sprintf("%s:%d", localhostAddress, magicPort+8*id+4)
		setupMC[id] = fmt.Sprintf("%s:%d", localhostAddress, magicPort+8*id+5)
		setupFetch[id] = fmt.Sprintf("%s:%d", localhostAddress, magicPort+8*id+6)
		setupRMC[id] = fmt.Sprintf("%s:%d", localhostAddress, magicPort+8*id+7)
	}
	return
}

func createAndStartProcess(
	id uint16,

	gossipAddresses []string,
	mcAddresses []string,
	fetchAddresses []string,
	rmcAddresses []string,

	setupGossipAddresses []string,
	setupMCAddresses []string,
	setupFetchAddresses []string,
	setupRMCAddresses []string,

	pubKeys []gomel.PublicKey,
	privKey gomel.PrivateKey,
	verificationKeys []*bn256.VerificationKey,
	secretKey *bn256.SecretKey,
	p2pPubKeys []*p2p.PublicKey,
	p2pSecKey *p2p.SecretKey,
	numberOfPreblocks int,
	dagFinished *sync.WaitGroup,
	finished *sync.WaitGroup,
) error {
	member := config.Member{
		Pid:          id,
		PrivateKey:   privKey,
		RMCSecretKey: secretKey,
		P2PSecretKey: p2pSecKey,
	}
	committee := config.Committee{
		PublicKeys:          pubKeys,
		RMCVerificationKeys: verificationKeys,
		P2PPublicKeys:       p2pPubKeys,
		SetupAddresses: map[string][]string{
			"mcast":  setupMCAddresses,
			"fetch":  setupFetchAddresses,
			"rmc":    setupRMCAddresses,
			"gossip": setupGossipAddresses,
		},
		Addresses: map[string][]string{
			"mcast":  mcAddresses,
			"fetch":  fetchAddresses,
			"rmc":    rmcAddresses,
			"gossip": gossipAddresses,
		},
	}
	cnf := config.New(&member, &committee)
	cnf.LogHuman = true
	cnf.LogBuffer = 100000
	cnf.LogLevel = 0
	// set stop condition for a process
	preblocksCountEstimation := numberOfPreblocks / cnf.EpochLength
	if preblocksCountEstimation == 0 {
		preblocksCountEstimation = 1
	}
	cnf.NumberOfEpochs = preblocksCountEstimation

	setupCnf := config.NewSetup(&member, &committee)
	cnf.OrderStartLevel = 6
	setupCnf.LogFile = "setup" + strconv.Itoa(int(id))
	setupCnf.LogHuman = cnf.LogHuman
	setupCnf.LogBuffer = 100000
	setupCnf.LogLevel = 0

	logConfig := cnf
	logConfig.LogFile = "cons" + strconv.Itoa(int(id))
	log, err := logging.NewLogger(logConfig)
	if err != nil {
		return err
	}

	// Mock data source and preblock sink.
	tds := tests.RandomDataSource(10)
	ps := make(chan *core.Preblock)

	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		tests.NopPreblockConsumer(ps)
	}()

	go func() {
		defer finished.Done()

		start, stop, err := run.Process(setupCnf, cnf, tds, ps)
		if err != nil {
			log.Err(err).Msg("failed to initialize a process")
			panic(err)
		}
		start()

		// wait for all expected preblocks
		wait.Wait()

		dagFinished.Done()
		dagFinished.Wait()

		stop()
	}()
	return nil
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
			if int(service.(float64)) == logging.ExtenderService {
				if event, ok := data[logging.Message]; ok {
					if event.(string) == logging.UnitOrdered {
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
	numberOfPreblocks := flag.Int("max_level", 12, "number of pre-blocks after which a process should finish; default is 12")
	flag.Parse()

	nProc := uint16(*testSize)

	gossipAddresses, mcAddresses, fetchAddresses, rmcAddresses, setupGossipAddresses, setupMCAddresses, setupFetchAddresses, setupRMCAddresses := generateLocalhostAddresses("localhost", *testSize)
	pubKeys, privKeys := generateKeys(nProc)
	sekKeys, verKeys := generateRMCKeys(nProc)
	p2pPubKeys, p2pSecKeys := generateP2PKeys(nProc)

	var allDone sync.WaitGroup
	var dagsFinished sync.WaitGroup
	dagsFinished.Add(len(gossipAddresses))
	for id := range gossipAddresses {
		allDone.Add(1)
		err := createAndStartProcess(
			uint16(id),

			gossipAddresses,
			mcAddresses,
			fetchAddresses,
			rmcAddresses,

			setupGossipAddresses,
			setupMCAddresses,
			setupFetchAddresses,
			setupRMCAddresses,

			pubKeys, privKeys[id], verKeys, sekKeys[id], p2pPubKeys, p2pSecKeys[id],
			*numberOfPreblocks,
			&dagsFinished,
			&allDone,
		)
		if err != nil {
			panic(err)
		}
	}

	// wait for all processes to finish
	allDone.Wait()
	// Sanity checks
	if checkOrderingFromLogs(nProc, "setup") {
		fmt.Println("Ordering in setup OK")
	} else {
		fmt.Println("Processes obtained different orderings in setup!")
		os.Exit(1)
	}
	if checkOrderingFromLogs(nProc, "cons") {
		fmt.Println("Ordering in main is OK")
	} else {
		fmt.Println("Processes obtained different orderings in main!")
		os.Exit(1)
	}
}
