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
	rsCh <- urn.NewUrn(config.Create.Pid)
	close(rsCh)
}

// BeaconSetup is a setup described in the whitepaper
func BeaconSetup(config process.Config, rsCh chan<- gomel.RandomSource, log zerolog.Logger) {
	dagFinished := make(chan struct{})
	var services []process.Service
	// attemptTimingRequests is a channel shared between orderer and creator/syncer
	// creator/syncer should send a notification to the channel when a new prime unit is added to the dag
	// orderer attempts timing decision after receiving the notification
	attemptTimingRequests := make(chan int)
	// orderedUnits is a channel orderer sends ordered units to
	//
	orderedUnits := make(chan gomel.Unit, 2*config.Dag.NProc())
	// txChan is a channel shared between tx_generator and creator
	txChan := make(chan []byte, 10)

	dag := growing.NewDag(config.Dag)
	rs := beacon.NewBeacon(config.Create.Pid)
	rs.Init(dag)

	service, err := create.NewService(dag, rs, config.CreateSetup, dagFinished, attemptTimingRequests, txChan, log.With().Int(logging.Service, logging.CreateService).Logger())
	if err != nil {
		close(rsCh)
		return
	}
	services = append(services, service)

	service, err = order.NewService(dag, rs, config.Order, attemptTimingRequests, orderedUnits, log.With().Int(logging.Service, logging.OrderService).Logger())
	if err != nil {
		close(rsCh)
		return
	}
	services = append(services, service)

	// We shouldn't have txs in the setup phase. But for now it stays.
	service, err = generate.NewService(dag, config.TxGenerate, txChan, log.With().Int(logging.Service, logging.GenerateService).Logger())
	if err != nil {
		close(rsCh)
		return
	}
	services = append(services, service)

	service, err = logging.NewService(config.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())
	if err != nil {
		close(rsCh)
		return
	}
	services = append(services, service)

	syncService, err := sync.NewService(dag, rs, config.SyncSetup, attemptTimingRequests, log.With().Int(logging.Service, logging.SyncService).Logger())
	if err != nil {
		close(rsCh)
		return
	}
	services = append(services, syncService)

	err = startAll(services)
	if err != nil {
		close(rsCh)
		return
	}

	u, ok := <-orderedUnits
	if !ok {
		close(rsCh)
	}
	rsCh <- rs.GetCoin(u.Creator())
	// Read and ignore the rest of orderedUnits
	go func() {
		for range orderedUnits {
		}
	}()
	close(rsCh)
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
