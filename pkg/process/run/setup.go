package run

import (
	"time"

	"github.com/rs/zerolog"

	chdag "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/parallel"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/process/create"
	"gitlab.com/alephledger/consensus-go/pkg/process/order"
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
	dag := chdag.New(nProc)
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
	orderedUnits := make(chan []gomel.Unit, 5)

	dag := makeBeaconDag(config.Dag)
	rs := beacon.New(config.Create.Pid)
	dag = rs.Bind(dag)

	orderService, orderingDag := order.NewService(dag, rs, config.OrderSetup, orderedUnits, log.With().Int(logging.Service, logging.OrderService).Logger())

	addService := &parallel.Parallel{}
	orderingAdder := addService.Register(orderingDag)
	addService.Start()
	defer addService.Stop()

	rmcService, rmcDag, err := rmc.NewService(orderingDag, orderingAdder, config.RMC, log)
	if err != nil {
		return
	}
	rmcAdder := addService.Register(rmcDag)
	createService, err := create.NewService(rmcDag, rmcAdder, rs, config.CreateSetup, dagFinished, nil, log.With().Int(logging.Service, logging.CreateService).Logger())
	if err != nil {
		return
	}
	memlogService, err := logging.NewService(config.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())
	if err != nil {
		return
	}
	services := []process.Service{createService, orderService, memlogService, rmcService}

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
		log.Info().Int(logging.Service, logging.ValidateService).Uint16(logging.Creator, u.Creator()).Int(logging.Height, u.Height()).Msg(logging.DataValidated)
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
	rmcService.Stop()
	<-dagFinished
}
