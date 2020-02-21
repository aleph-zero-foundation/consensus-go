// Package run defines a function for running the whole protocol, using services defined in other packages.
package run

import (
	"errors"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	dagutils "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/forking"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/order"
	"gitlab.com/alephledger/consensus-go/pkg/sync/syncer"
	"gitlab.com/alephledger/core-go/pkg/core"
	"gitlab.com/alephledger/core-go/pkg/crypto/tss"
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

func newPreblockService(orderedUnits chan []gomel.Unit, ps core.PreblockSink) gomel.Service {
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
	return beaconSetup(conf, randomSourceSink, setupLog, fatalError)
}

func NewConsensus(
	conf config.Config,
	ds core.DataSource,
	ps core.PreblockSink,
	wtkSource <-chan tss.WeakThresholdKey,
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

				// orderedUnits is a channel shared between orderer and validator
				// orderer sends ordered rounds to the channel
				orderedUnits := make(chan []gomel.Unit, 10)
				defer close(orderedUnits)

				var wtk tss.WeakThresholdKey
				select {
				case wtk = <-wtkSource:
				case <-stopService:
					return
				}
				log.Info().Msg(logging.GotWeakThresholdKey)

				rsf := 

				orderer := order.NewOrderer(conf, rsf, ps)

				alerter, err :=
					forking.NewAlertService(conf, orderer, log.With().Int(logging.Service, logging.AlertService).Logger())
				if err != nil {
					log.Err(err).Msg("initialization of the alerter service failed")
					fatalError <- err
					return
				}
				orderer.SetAlerter(alerter)

				syncer, err = syncer.New(conf, orderer, log)
				if err != nil {
					log.Err(err).Msg("initialization of the sync service failed")
					fatalError <- err
					return
				}
				orderer.SetSyncer(syncer)

				memlogService :=
					logging.NewService(conf.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())

				err = start(
					alerter,
					memlogService,
					syncer,
				)
				if err != nil {
					log.Err(err).Msg("failed to start main services")
					fatalError <- err
					return
				}
				defer stop(adderService, createService, extender, memlogService, preblockService)

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
			stop(alerter, syncer)
		},
	), nil
}

// Process creates an default instance of the Orderer service using provided configuration.
func Process(
	conf config.Config,
	ds core.DataSource,
	ps core.PreblockSink,
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
