package run

import (
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/adder"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	dagutils "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/random/beacon"
	"gitlab.com/alephledger/consensus-go/pkg/random/coin"
	"gitlab.com/alephledger/consensus-go/pkg/services/create"
	"gitlab.com/alephledger/consensus-go/pkg/services/order"
	"gitlab.com/alephledger/consensus-go/pkg/services/sync"
)

// coinSetup deals a coin. Running a process with this setup
// is equivalent to the version without a setup phase.
func coinSetup(conf config.Config, rsCh chan<- gomel.RandomSource, log zerolog.Logger) {
	pid := conf.Create.Pid
	nProc := conf.NProc

	shareProviders := make(map[uint16]bool)
	for i := uint16(0); i < nProc; i++ {
		shareProviders[i] = true
	}

	rsCh <- coin.NewFixedCoin(nProc, pid, 1234, shareProviders)
	close(rsCh)
}

func makeBeaconDag(nProc uint16) gomel.Dag {
	dag := dagutils.New(nProc)
	check.BasicCompliance(dag)
	check.ParentConsistency(dag)
	check.PrimeOnlyNoSkipping(dag)
	check.NoForks(dag)
	return dag
}

// beaconSetup is the setup described in the whitepaper.
func beaconSetup(conf config.Config, rsCh chan<- gomel.RandomSource, log zerolog.Logger) {
	defer close(rsCh)
	dagFinished := make(chan struct{})
	// orderedUnits is a channel shared between orderer and validator
	// orderer sends ordered rounds to the channel
	orderedUnits := make(chan []gomel.Unit, 10)

	dag := makeBeaconDag(conf.NProc)
	rs, err := beacon.New(conf.Create.Pid, conf.P2PPublicKeys, conf.P2PSecretKey)
	if err != nil {
		log.Error().Str("where", "setup.beacon.New").Msg(err.Error())
	}
	rs.Bind(dag)

	adr, adderService := adder.New(dag, conf.PublicKeys, log.With().Int(logging.Service, logging.AdderService).Logger())

	orderService := order.NewService(dag, rs, conf.Order, orderedUnits, log.With().Int(logging.Service, logging.OrderService).Logger())

	syncService, err := sync.NewService(dag, adr, conf.Sync, log)
	if err != nil {
		log.Error().Str("where", "setup.sync").Msg(err.Error())
		return
	}

	createService := create.NewService(dag, adr, rs, conf.CreateSetup, dagFinished, nil, log.With().Int(logging.Service, logging.CreateService).Logger())

	memlogService := logging.NewService(conf.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())

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
