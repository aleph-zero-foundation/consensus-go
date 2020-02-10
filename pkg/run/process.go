// Package run defines a function for running the whole protocol, using services defined in other packages.
package run

import (
	"errors"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/adder"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	dagutils "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/services/alert"
	"gitlab.com/alephledger/consensus-go/pkg/services/create"
	"gitlab.com/alephledger/consensus-go/pkg/services/order"
	"gitlab.com/alephledger/consensus-go/pkg/services/sync"
)

type closureService struct {
	startClosure func() error
	stopClosure  func()
}

func newClosureService(startClosure func() error, stopClosure func()) *closureService {
	return &closureService{startClosure: startClosure, stopClosure: stopClosure}
}

func (ds *closureService) Start() error {
	return ds.startClosure()
}

func (ds *closureService) Stop() {
	ds.stopClosure()
}

func stop(services ...gomel.Service) {
	for _, s := range services {
		s.Stop()
	}
}

func start(services ...gomel.Service) error {
	for i, s := range services {
		err := s.Start()
		if err != nil {
			stop(services[:i]...)
			return err
		}
	}
	return nil
}

func newPreblockService(orderedUnits chan []gomel.Unit, ps gomel.PreblockSink) gomel.Service {
	var stopService, serviceFinished chan struct{}
	return newClosureService(
		func() error {
			stopService = make(chan struct{})
			serviceFinished = make(chan struct{})
			go func() {
				defer close(serviceFinished)
				for {
					var round []gomel.Unit
					select {
					case round = <-orderedUnits:
					case <-stopService:
						return
					}
					select {
					case ps <- gomel.ToPreblock(round):
					case <-stopService:
						return
					}
				}
			}()
			return nil
		},

		func() {
			close(stopService)
			<-serviceFinished
		})
}

func makeStandardDag(nProc uint16) gomel.Dag {
	dag := dagutils.New(nProc)
	check.BasicCompliance(dag)
	check.ParentConsistency(dag)
	check.NoSelfForkingEvidence(dag)
	check.ForkerMuting(dag)
	return dag
}

// BeaconSetup returns an instance of the Service type that implements the setup procedure of gomel, i.e. it attempts
// to construct an instance of RandomSource.
func BeaconSetup(
	conf config.Config,
	setupLog zerolog.Logger,
	randomSourceSink chan<- func(gomel.Dag) gomel.RandomSource,
	fatalError chan error,
) (gomel.Service, error) {
	var setup func(config.Config, chan<- func(gomel.Dag) gomel.RandomSource, zerolog.Logger, chan error) (gomel.Service, error)
	if conf.Setup == "coin" {
		setup = coinSetup
	}
	if conf.Setup == "beacon" {
		setup = beaconSetup
	}
	if setup == nil {
		return nil, errors.New("unknown type of setup procedure: " + conf.Setup)
	}
	return setup(conf, randomSourceSink, setupLog, fatalError)
}

func newConsensus(
	conf config.Config,
	ds gomel.DataSource,
	ps gomel.PreblockSink,
	rsSource <-chan func(gomel.Dag) gomel.RandomSource,
	createdDag chan<- gomel.Dag,
	log zerolog.Logger,
	fatalError chan error,
) (gomel.Service, error) {

	var serviceStopped, stopService chan struct{}
	var alertService, syncService gomel.Service

	return newClosureService(
		func() error {
			serviceStopped = make(chan struct{})
			stopService = make(chan struct{})

			go func() {
				defer close(serviceStopped)

				dagFinished := make(chan struct{})
				defer close(dagFinished)
				// orderedUnits is a channel shared between orderer and validator
				// orderer sends ordered rounds to the channel
				orderedUnits := make(chan []gomel.Unit, 10)
				defer close(orderedUnits)
				dag := makeStandardDag(conf.NProc)
				defer func() {
					createdDag <- dag
				}()

				var rsProvider func(gomel.Dag) gomel.RandomSource
				select {
				case rsProvider = <-rsSource:
				case <-stopService:
					return
				}
				rs := rsProvider(dag)
				log.Info().Msg(logging.GotRandomSource)

				var err error
				var alerter gomel.Alerter
				alerter, alertService, err =
					alert.NewService(dag, conf.Alert, log.With().Int(logging.Service, logging.AlertService).Logger())
				if err != nil {
					log.Err(err).Msg("main service's initialization failed")
					fatalError <- err
					return
				}

				adr, adderService :=
					adder.New(dag, alerter, conf.PublicKeys, log.With().Int(logging.Service, logging.AdderService).Logger())

				orderService :=
					order.NewService(
						dag,
						rs,
						conf.Order,
						orderedUnits,
						log.With().Int(logging.Service, logging.OrderService).Logger(),
					)

				syncService, err = sync.NewService(dag, adr, conf.Sync, log)
				if err != nil {
					log.Err(err).Msg("initialization of the sync service failed")
					fatalError <- err
					return
				}

				createService :=
					create.NewService(
						dag,
						adr,
						rs,
						conf.Create,
						dagFinished,
						ds,
						log.With().Int(logging.Service, logging.CreateService).Logger())

				memlogService :=
					logging.NewService(conf.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())

				preblockService := newPreblockService(orderedUnits, ps)

				err = start(
					alertService,
					adderService,
					createService,
					orderService,
					memlogService,
					syncService,
					preblockService,
				)
				if err != nil {
					log.Err(err).Msg("failed to start main services")
					fatalError <- err
					return
				}
				defer stop(adderService, createService, orderService, memlogService, preblockService)

				select {
				case <-dagFinished:
				case <-stopService:
					return
				}
			}()
			return nil
		},

		func() {
			close(stopService)
			<-serviceStopped
			stop(alertService, syncService)
		},
	), nil
}

// NewConsensus returns a service that starts all main components of gomel capable of producing a stream of ordered Preblocks.
func NewConsensus(
	conf config.Config,
	ds gomel.DataSource,
	ps gomel.PreblockSink,
	rsSource <-chan func(gomel.Dag) gomel.RandomSource,
	log zerolog.Logger,
	fatalError chan error,
) (gomel.Service, error) {
	createdDag := make(chan gomel.Dag, 1)
	return newConsensus(conf, ds, ps, rsSource, createdDag, log, fatalError)
}

// Process creates an default instance of the Orderer service using provided configuration.
func Process(
	conf config.Config,
	ds gomel.DataSource,
	ps gomel.PreblockSink,
	createdDag chan<- gomel.Dag,
	setupLog zerolog.Logger,
	log zerolog.Logger,
	setupError,
	mainError chan error,
) (gomel.Service, error) {

	// rsSource is a channel shared between setup process and the main process.
	// The setup process should create a random source and push it to the channel.
	// The main process waits on the channel.
	var rsSource chan func(gomel.Dag) gomel.RandomSource
	var setupService, mainService gomel.Service

	return newClosureService(
		func() error {
			rsSource = make(chan func(gomel.Dag) gomel.RandomSource, 1)
			var err error
			setupService, err = BeaconSetup(conf, setupLog, rsSource, setupError)
			if err != nil {
				return errors.New("error while initializing setup service: " + err.Error())
			}

			mainService, err = newConsensus(conf, ds, ps, rsSource, createdDag, log, mainError)
			if err != nil {
				return err
			}

			err = setupService.Start()
			if err != nil {
				return errors.New("error while starting the setup service: " + err.Error())
			}
			err = mainService.Start()
			if err != nil {
				setupService.Stop()
				return errors.New("error while starting the main service: " + err.Error())
			}
			return nil
		},

		func() {
			setupService.Stop()
			mainService.Stop()
			close(rsSource)
		},
	), nil
}
