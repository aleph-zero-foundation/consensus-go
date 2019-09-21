package run

import (
	"time"

	"github.com/rs/zerolog"

	dagutils "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/parallel"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/process/create"
	"gitlab.com/alephledger/consensus-go/pkg/process/order"
	"gitlab.com/alephledger/consensus-go/pkg/process/sync"
	"gitlab.com/alephledger/consensus-go/pkg/random/beacon"
	"gitlab.com/alephledger/consensus-go/pkg/random/coin"
)

// coinSetup deals a coin. Running a process with this setup
// is equivalent to the version without a setup phase.
func coinSetup(config process.Config, rsCh chan<- gomel.RandomSource, log zerolog.Logger) {
	pid := config.Create.Pid
	nProc := uint16(len(config.Dag.Keys))
	rsCh <- coin.NewFixedCoin(nProc, pid, 1234)
	close(rsCh)
}

func makeBeaconDag(conf *gomel.DagConfig) gomel.Dag {
	nProc := uint16(len(conf.Keys))
	dag := dagutils.New(nProc)
	dag, _ = check.Signatures(dag, conf.Keys)
	dag = check.BasicCompliance(dag)
	dag = check.ParentDiversity(dag)
	dag = check.PrimeOnlyNoSkipping(dag)
	dag = check.NoForks(dag)
	return dag
}

// beaconSetup is the setup described in the whitepaper.
func beaconSetup(config process.Config, rsCh chan<- gomel.RandomSource, log zerolog.Logger) {
	defer close(rsCh)
	dagFinished := make(chan struct{})
	// orderedUnits is a channel shared between orderer and validator
	// orderer sends ordered rounds to the channel
	orderedUnits := make(chan []gomel.Unit, 10)
	txChan := (chan []byte)(nil)

	dag := makeBeaconDag(config.Dag)
	rs := beacon.New(config.Create.Pid)

	// common with main:
	dag = rs.Bind(dag)

	orderService, orderIfPrime := order.NewService(dag, rs, config.OrderSetup, orderedUnits, log.With().Int(logging.Service, logging.OrderService).Logger())
	dag = dagutils.AfterEmplace(dag, orderIfPrime)

	adderService := &parallel.Parallel{}
	adder := adderService.Register(dag)

	syncService, multicastUnit, err := sync.NewService(dag, adder, config.SyncSetup, log)
	if err != nil {
		//TODO silenced error
		return
	}
	dagMC := dagutils.AfterEmplace(dag, multicastUnit)
	adderMC := adderService.Register(dagMC)

	createService := create.NewService(dagMC, adderMC, rs, config.CreateSetup, dagFinished, txChan, log.With().Int(logging.Service, logging.CreateService).Logger())

	memlogService := logging.NewService(config.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())
	// end common

	services := []process.Service{adderService, createService, orderService, memlogService, syncService}

	err = startAll(services)
	if err != nil {
		//TODO silenced error
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
		log.Info().Int(logging.Service, logging.ValidateService).Uint16(logging.Creator, u.Creator()).Int(logging.Height, u.Height()).Msg(logging.DataValidated)
	}
	// Read and ignore the rest of orderedUnits
	go func() {
		for range orderedUnits {
		}
	}()
	// we should still sync with each other

	// We need to figure out a condition for stopping the setup phase syncs
	// For now just syncing for the next minute
	stopAll(services[len(services)-1:])
	time.Sleep(60 * time.Second)
	syncService.Stop()
	<-dagFinished
}
