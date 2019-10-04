package run

import (
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	dagutils "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/parallel"
	"gitlab.com/alephledger/consensus-go/pkg/process/create"
	"gitlab.com/alephledger/consensus-go/pkg/process/order"
	"gitlab.com/alephledger/consensus-go/pkg/process/sync"
	"gitlab.com/alephledger/consensus-go/pkg/random/beacon"
	"gitlab.com/alephledger/consensus-go/pkg/random/coin"
)

// coinSetup deals a coin. Running a process with this setup
// is equivalent to the version without a setup phase.
func coinSetup(conf config.Config, rsCh chan<- gomel.RandomSource, log zerolog.Logger) {
	pid := conf.Create.Pid
	nProc := uint16(len(conf.Dag.Keys))
	rsCh <- coin.NewFixedCoin(nProc, pid, 1234)
	close(rsCh)
}

func makeBeaconDag(conf *config.Dag) gomel.Dag {
	nProc := uint16(len(conf.Keys))
	dag := dagutils.New(nProc)
	dag, _ = check.Signatures(dag, conf.Keys)
	dag = check.BasicCompliance(dag)
	dag = check.ParentConsistency(dag)
	dag = check.PrimeOnlyNoSkipping(dag)
	dag = check.NoForks(dag)
	return dag
}

// beaconSetup is the setup described in the whitepaper.
func beaconSetup(conf config.Config, rsCh chan<- gomel.RandomSource, log zerolog.Logger) {
	defer close(rsCh)
	dagFinished := make(chan struct{})
	// orderedUnits is a channel shared between orderer and validator
	// orderer sends ordered rounds to the channel
	orderedUnits := make(chan []gomel.Unit, 10)
	txChan := (chan []byte)(nil)

	dag := makeBeaconDag(conf.Dag)
	rs := beacon.New(conf.Create.Pid)

	// common with main:
	dag = rs.Bind(dag)

	orderService, orderIfPrime := order.NewService(dag, rs, conf.OrderSetup, orderedUnits, log.With().Int(logging.Service, logging.OrderService).Logger())
	dag = dagutils.AfterEmplace(dag, orderIfPrime)

	adderService := &parallel.Parallel{}
	adder := adderService.Register(dag)

	syncService, multicastUnit, err := sync.NewService(dag, adder, conf.SyncSetup, log)
	if err != nil {
		log.Error().Str("where", "setup.sync").Msg(err.Error())
		return
	}
	dagMC := dagutils.AfterEmplace(dag, multicastUnit)
	adderMC := adderService.Register(dagMC)

	createService := create.NewService(dagMC, adderMC, rs, conf.CreateSetup, dagFinished, txChan, log.With().Int(logging.Service, logging.CreateService).Logger())

	memlogService := logging.NewService(conf.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())
	// end common

	err = start(adderService, createService, orderService, memlogService, syncService)
	if err != nil {
		log.Error().Str("where", "setup.start").Msg(err.Error())
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

	// We need to figure out a condition for stopping the setup phase syncs
	// For now just syncing for some time
	stop(createService, orderService, memlogService)
	time.Sleep(10 * time.Second)
	stop(syncService, adderService)
	<-dagFinished
}
