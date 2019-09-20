// Package run defines a function for running the whole protocol, using services defined in other packages.
package run

import (
	"errors"

	"github.com/rs/zerolog"

	chdag "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/parallel"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/process/create"
	"gitlab.com/alephledger/consensus-go/pkg/process/order"
	"gitlab.com/alephledger/consensus-go/pkg/process/sync"
	"gitlab.com/alephledger/consensus-go/pkg/process/tx/generate"
	"gitlab.com/alephledger/consensus-go/pkg/process/tx/validate"
)

func stopAll(services []process.Service) {
	for _, s := range services {
		s.Stop()
	}
}

func startAll(services []process.Service) error {
	for i, s := range services {
		err := s.Start()
		if err != nil {
			stopAll(services[:i])
			return err
		}
	}
	return nil
}

func makeStandardDag(conf *gomel.DagConfig) gomel.Dag {
	nProc := uint16(len(conf.Keys))
	dag := chdag.New(nProc)
	dag, _ = check.Signatures(dag, conf.Keys)
	dag = check.BasicCompliance(dag)
	dag = check.ParentDiversity(dag)
	dag = check.NoSelfForkingEvidence(dag)
	dag = check.ForkerMuting(dag)
	dag = check.ExpandPrimes(dag)
	return dag
}

// Process starts the main and setup processes.
func Process(config process.Config, setupLog zerolog.Logger, log zerolog.Logger) (gomel.Dag, error) {
	// rsCh is a channel shared between setup process and the main process.
	// The setup process should create a random source and push it to the channel.
	// The main process waits on the channel.
	rsCh := make(chan gomel.RandomSource)

	if config.Setup == "coin" {
		go coinSetup(config, rsCh, setupLog)
	}
	if config.Setup == "beacon" {
		go beaconSetup(config, rsCh, setupLog)
	}
	return main(config, rsCh, log)
}

// main runs all the services with the configuration provided.
// It blocks until all of them are done.
func main(config process.Config, rsCh <-chan gomel.RandomSource, log zerolog.Logger) (gomel.Dag, error) {
	dagFinished := make(chan struct{})
	// orderedUnits is a channel shared between orderer and validator
	// orderer sends ordered rounds to the channel
	orderedUnits := make(chan []gomel.Unit, 10)
	// txChan is a channel shared between tx_generator and creator
	txChan := make(chan []byte, 10)

	dag := makeStandardDag(config.Dag)
	rs, ok := <-rsCh
	if !ok {
		return nil, errors.New("setup phase failed")
	}

	// common with setup:
	dag = rs.Bind(dag)

	orderService, orderIfPrime := order.NewService(dag, rs, config.Order, orderedUnits, log.With().Int(logging.Service, logging.OrderService).Logger())
	dag = chdag.AfterEmplace(dag, orderIfPrime)

	adderService := &parallel.Parallel{}
	adder := adderService.Register(dag)

	syncService, multicastUnit, err := sync.NewService(dag, adder, config.Sync, log)
	if err != nil {
		return nil, err
	}
	dagMC := chdag.AfterEmplace(dag, multicastUnit)
	adderMC := adderService.Register(dagMC)

	createService := create.NewService(dagMC, adderMC, rs, config.Create, dagFinished, txChan, log.With().Int(logging.Service, logging.CreateService).Logger())

	memlogService := logging.NewService(config.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())
	// end common

	validateService := validate.NewService(config.TxValidate, orderedUnits, log.With().Int(logging.Service, logging.ValidateService).Logger())
	generateService := generate.NewService(config.TxGenerate, txChan, log.With().Int(logging.Service, logging.GenerateService).Logger())

	services := []process.Service{adderService, createService, orderService, generateService, validateService, memlogService, syncService}

	err = startAll(services)
	if err != nil {
		return nil, err
	}
	defer stopAll(services)
	<-dagFinished
	return dag, nil
}
