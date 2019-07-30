package run

import (
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/process/create"
	"gitlab.com/alephledger/consensus-go/pkg/process/order"
	"gitlab.com/alephledger/consensus-go/pkg/process/sync"
	"gitlab.com/alephledger/consensus-go/pkg/process/tx/generate"
	"gitlab.com/alephledger/consensus-go/pkg/random/beacon"
	"gitlab.com/alephledger/consensus-go/pkg/random/urn"
)

// UrnSetup just deals an urn. Running a process with this setup
// is equivalent to the version of without setup phase.
func UrnSetup(config process.Config, rsCh chan<- gomel.RandomSource, log zerolog.Logger) {
	rsCh <- urn.New(config.Create.Pid)
	close(rsCh)
}

// BeaconSetup is a setup described in the whitepaper
func BeaconSetup(config process.Config, rsCh chan<- gomel.RandomSource, log zerolog.Logger) {
	defer close(rsCh)
	dagFinished := make(chan struct{})
	// orderedUnits is a channel shared between orderer and validator
	// orderer sends ordered rounds to the channel
	orderedUnits := make(chan []gomel.Unit, 5)
	// txChan is a channel shared between tx_generator and creator
	txChan := make(chan []byte, 10)

	dag := growing.NewDag(config.Dag)
	rs := beacon.New(config.Create.Pid)
	rs.Init(dag)

	orderService, primeAlert, err := order.NewService(dag, rs, config.OrderSetup, orderedUnits, log.With().Int(logging.Service, logging.OrderService).Logger())
	if err != nil {
		return
	}
	syncService, requestMulticast, err := sync.NewService(dag, rs, config.SyncSetup, primeAlert, log)
	if err != nil {
		return
	}
	createService, err := create.NewService(dag, rs, config.CreateSetup, dagFinished, gomel.MergeCallbacks(requestMulticast, primeAlert), txChan, log.With().Int(logging.Service, logging.CreateService).Logger())
	if err != nil {
		return
	}
	generateService, err := generate.NewService(dag, config.TxGenerate, txChan, log.With().Int(logging.Service, logging.GenerateService).Logger())
	if err != nil {
		return
	}
	memlogService, err := logging.NewService(config.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())
	if err != nil {
		return
	}
	services := []process.Service{createService, orderService, generateService, memlogService, syncService}

	err = startAll(services)
	if err != nil {
		return
	}

	units, ok := <-orderedUnits
	if !ok || len(units) == 0 {
		return
	}
	head := units[len(units)-1]
	rsCh <- rs.GetCoin(head.Creator())
	// logging the order
	for _, u := range units {
		log.Info().Int(logging.Service, logging.ValidateService).Int(logging.Creator, u.Creator()).Int(logging.Height, u.Height()).Msg(logging.DataValidated)
	}
	// Read and ignore the rest of orderedUnits
	go func() {
		for range orderedUnits {
		}
	}()
	// we should still sync with each other
	services = services[:(len(services) - 1)]
	stopAll(services)

	// We need to figure out a condition for stopping the setup phase syncs
	// For now just syncing for the next minute
	time.Sleep(60 * time.Second)
	syncService.Stop()
	<-dagFinished
	dag.Stop()
}
