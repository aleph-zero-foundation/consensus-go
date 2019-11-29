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

func makeStandardDag(nProc uint16) gomel.Dag {
	dag := dagutils.New(nProc)
	check.BasicCompliance(dag)
	check.ParentConsistency(dag)
	check.NoSelfForkingEvidence(dag)
	check.ForkerMuting(dag)
	return dag
}

// Process starts the main and setup processes.
func Process(conf config.Config, ds gomel.DataSource, ps gomel.PreblockSink, setupLog zerolog.Logger, log zerolog.Logger) (gomel.Dag, error) {
	// rsCh is a channel shared between setup process and the main process.
	// The setup process should create a random source and push it to the channel.
	// The main process waits on the channel.
	rsCh := make(chan gomel.RandomSource)

	if conf.Setup == "coin" {
		go coinSetup(conf, rsCh, setupLog)
	}
	if conf.Setup == "beacon" {
		go beaconSetup(conf, rsCh, setupLog)
	}
	return main(conf, ds, ps, rsCh, log)
}

// main runs all the services with the configuration provided.
// It blocks until all of them are done.
func main(conf config.Config, ds gomel.DataSource, ps gomel.PreblockSink, rsCh <-chan gomel.RandomSource, log zerolog.Logger) (gomel.Dag, error) {
	dagFinished := make(chan struct{})
	// orderedUnits is a channel shared between orderer and validator
	// orderer sends ordered rounds to the channel
	orderedUnits := make(chan []gomel.Unit, 10)
	dag := makeStandardDag(conf.NProc)

	rs, ok := <-rsCh
	if !ok {
		return nil, errors.New("setup phase failed")
	}
	log.Info().Msg(logging.GotRandomSource)
	rs.Bind(dag)

	alerter, alertService, err := alert.NewService(dag, conf.Alert, log.With().Int(logging.Service, logging.AlertService).Logger())
	if err != nil {
		return nil, err
	}

	adr, adderService, setFetch, setGossip := adder.New(dag, alerter, conf.PublicKeys, log.With().Int(logging.Service, logging.AdderService).Logger())

	orderService := order.NewService(dag, rs, conf.Order, orderedUnits, log.With().Int(logging.Service, logging.OrderService).Logger())

	syncService, err := sync.NewService(dag, adr, conf.Sync, setFetch, setGossip, log)
	if err != nil {
		return nil, err
	}

	createService := create.NewService(dag, adr, rs, conf.Create, dagFinished, ds, log.With().Int(logging.Service, logging.CreateService).Logger())

	memlogService := logging.NewService(conf.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())

	go func() {
		for round := range orderedUnits {
			ps <- gomel.ToPreblock(round)
		}
	}()

	err = start(alertService, adderService, createService, orderService, memlogService, syncService)
	if err != nil {
		return nil, err
	}
	<-dagFinished
	stop(createService, orderService, memlogService, syncService, adderService, alertService)
	return dag, nil
}
