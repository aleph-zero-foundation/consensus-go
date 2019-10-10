// Package run defines a function for running the whole protocol, using services defined in other packages.
package run

import (
	"errors"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	dagutils "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/parallel"
	"gitlab.com/alephledger/consensus-go/pkg/services/alert"
	"gitlab.com/alephledger/consensus-go/pkg/services/create"
	"gitlab.com/alephledger/consensus-go/pkg/services/order"
	"gitlab.com/alephledger/consensus-go/pkg/services/sync"
	"gitlab.com/alephledger/consensus-go/pkg/services/tx/generate"
	"gitlab.com/alephledger/consensus-go/pkg/services/tx/validate"
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

func makeStandardDag(conf *config.Dag) gomel.Dag {
	nProc := uint16(len(conf.Keys))
	dag := dagutils.New(nProc)
	dag, _ = check.Signatures(dag, conf.Keys)
	dag = check.BasicCompliance(dag)
	dag = check.ParentConsistency(dag)
	dag = check.NoSelfForkingEvidence(dag)
	dag = check.ForkerMuting(dag)
	return dag
}

// Process starts the main and setup processes.
func Process(conf config.Config, setupLog zerolog.Logger, log zerolog.Logger) (gomel.Dag, error) {
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
	return main(conf, rsCh, log)
}

// main runs all the services with the configuration provided.
// It blocks until all of them are done.
func main(conf config.Config, rsCh <-chan gomel.RandomSource, log zerolog.Logger) (gomel.Dag, error) {
	dagFinished := make(chan struct{})
	// orderedUnits is a channel shared between orderer and validator
	// orderer sends ordered rounds to the channel
	orderedUnits := make(chan []gomel.Unit, 10)
	// txChan is a channel shared between tx_generator and creator
	txChan := make(chan []byte, 10)

	dag := makeStandardDag(conf.Dag)

	rs, ok := <-rsCh
	if !ok {
		return nil, errors.New("setup phase failed")
	}
	log.Info().Msg(logging.GotRandomSource)

	dag = rs.Bind(dag)

	dag, alertService, fetchData, err := alert.NewService(dag, conf.Alert, log.With().Int(logging.Service, logging.AlertService).Logger())
	if err != nil {
		return nil, err
	}

	orderService, orderIfPrime := order.NewService(dag, rs, conf.Order, orderedUnits, log.With().Int(logging.Service, logging.OrderService).Logger())
	dag = dagutils.AfterEmplace(dag, orderIfPrime)

	adderService := &parallel.Parallel{}
	adder := adderService.Register(dag)

	syncService, multicastUnit, err := sync.NewService(dag, adder, fetchData, conf.Sync, log)
	if err != nil {
		return nil, err
	}
	dagMC := dagutils.AfterEmplace(dag, multicastUnit)
	adderMC := adderService.Register(dagMC)

	createService := create.NewService(dagMC, adderMC, rs, conf.Create, dagFinished, txChan, log.With().Int(logging.Service, logging.CreateService).Logger())

	memlogService := logging.NewService(conf.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())

	validateService := validate.NewService(conf.TxValidate, orderedUnits, log.With().Int(logging.Service, logging.ValidateService).Logger())
	generateService := generate.NewService(conf.TxGenerate, txChan, log.With().Int(logging.Service, logging.GenerateService).Logger())

	err = start(alertService, adderService, createService, orderService, generateService, validateService, memlogService, syncService)
	if err != nil {
		return nil, err
	}
	<-dagFinished
	stop(createService, orderService, generateService, validateService, memlogService, syncService, adderService, alertService)
	return dag, nil
}
