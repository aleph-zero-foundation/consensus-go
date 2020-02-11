package run

import (
	"fmt"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/adder"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	dagutils "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/linear"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/random/beacon"
	"gitlab.com/alephledger/consensus-go/pkg/random/coin"
	"gitlab.com/alephledger/consensus-go/pkg/services/create"
	"gitlab.com/alephledger/consensus-go/pkg/services/sync"
)

// coinSetup deals a coin. Running a process with this setup
// is equivalent to the version without a setup phase.
func coinSetup(conf config.Config, rsCh chan<- func(gomel.Dag) gomel.RandomSource, log zerolog.Logger, fatalError chan error) (gomel.Service, error) {
	var stopService, serviceStopped chan struct{}

	return newClosureService(
		func() error {
			stopService = make(chan struct{})
			serviceStopped = make(chan struct{})

			go func() {
				defer close(serviceStopped)

				pid := conf.Create.Pid
				nProc := conf.NProc

				shareProviders := make(map[uint16]bool)
				for i := uint16(0); i < nProc; i++ {
					shareProviders[i] = true
				}
				select {
				case rsCh <- func(dag gomel.Dag) gomel.RandomSource {
					coin := coin.NewFixedCoin(nProc, pid, 1234, shareProviders)
					coin.Bind(dag)
					return coin
				}:
				case <-stopService:
				}
			}()

			return nil
		},
		func() {
			close(stopService)
			<-serviceStopped
		},
	), nil
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
func beaconSetup(conf config.Config, rsCh chan<- func(gomel.Dag) gomel.RandomSource, log zerolog.Logger, fatalError chan error) (gomel.Service, error) {
	var adderService, syncService gomel.Service

	var stopService, serviceStopped chan struct{}

	return newClosureService(
		func() error {
			stopService = make(chan struct{})
			serviceStopped = make(chan struct{})
			dagFinished := make(chan struct{})
			// orderedUnits is a channel shared between orderer and validator
			// orderer sends ordered rounds to the channel
			orderedUnits := make(chan []gomel.Unit, 10)

			dag := makeBeaconDag(conf.NProc)
			rs, err := beacon.New(conf.Create.Pid, conf.P2PPublicKeys, conf.P2PSecretKey)
			if err != nil {
				log.Error().Str("where", "setup.beacon.New").Msg(err.Error())
				return err
			}
			rs.Bind(dag)

			var adr gomel.Adder
			adr, adderService = adder.New(dag, gomel.NopAlerter(), conf.PublicKeys, log.With().Int(logging.Service, logging.AdderService).Logger())

			extender := linear.NewExtender(dag, rs, conf.OrderSetup, orderedUnits, log.With().Int(logging.Service, logging.OrderService).Logger())

			syncService, err = sync.NewService(dag, adr, conf.SyncSetup, log)
			if err != nil {
				log.Error().Str("where", "setup.sync").Msg(err.Error())
				return err
			}

			createService := create.NewService(dag, adr, rs, conf.CreateSetup, dagFinished, nil, log.With().Int(logging.Service, logging.CreateService).Logger())

			memlogService := logging.NewService(conf.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())

			err = start(adderService, createService, extender, memlogService, syncService)
			if err != nil {
				log.Error().Str("where", "setup.start").Msg(err.Error())
				return err
			}

			go func() {
				defer close(serviceStopped)
				defer func() {
					stop(createService, extender, memlogService)
					close(orderedUnits)
					close(dagFinished)
				}()

				var units []gomel.Unit
				select {
				case units = <-orderedUnits:
				case <-stopService:
					return
				}
				if len(units) == 0 {
					log.Error().Msg("setup failed: ordering service returned an empty list")
					return
				}
				head := units[len(units)-1]
				if head.Level() != conf.OrderSetup.OrderStartLevel {
					msg := fmt.Sprintf(
						"setup failed: ordering service returned a head from a wrong level: expected %d, received %d",
						conf.OrderSetup.OrderStartLevel,
						head.Level(),
					)
					log.Error().Msg(msg)
					fatalError <- err
					return
				}
				select {
				case rsCh <- func(dag gomel.Dag) gomel.RandomSource {
					coin := rs.GetCoin(head.Creator())
					coin.Bind(dag)
					return coin
				}:
				case <-stopService:
					return
				}

				for _, u := range units {
					log.Info().Int(logging.Service, logging.ValidateService).Uint16(logging.Creator, u.Creator()).Int(logging.Height, u.Height()).Msg(logging.DataValidated)
				}
			}()

			return nil
		},

		func() {
			close(stopService)
			<-serviceStopped
			stop(syncService, adderService)
		},
	), nil
}
