package run

import (
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/process/create"
	"gitlab.com/alephledger/consensus-go/pkg/process/order"
	"gitlab.com/alephledger/consensus-go/pkg/process/sync"
	"gitlab.com/alephledger/consensus-go/pkg/process/tx/generate"
	"gitlab.com/alephledger/consensus-go/pkg/process/tx/validate"
	"gitlab.com/alephledger/consensus-go/pkg/random/urn"
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

// Process runs all the services with the configuration provided.
// It blocks until all of them are done.
func Process(config process.Config, log zerolog.Logger) (gomel.Dag, error) {
	dagFinished := make(chan struct{})
	var services []process.Service
	// attemptTimingRequests is a channel shared between orderer and creator/syncer
	// creator/syncer should send a notification to the channel when a new prime unit is added to the dag
	// orderer attempts timing decision after receiving the notification
	attemptTimingRequests := make(chan int)
	// orderedUnits is a channel shared between orderer and validator
	// orderer sends ordered units to the channel
	// validator reads the units from the channel and validates transactions contained in the unit
	// We expect to order about one level of units at once, which should be around the size of the committee.
	// The buffer has size taking that into account with some leeway.
	orderedUnits := make(chan gomel.Unit, 2*config.Dag.NProc())
	// txChan is a channel shared between tx_generator and creator
	txChan := make(chan []byte, 10)

	dag := growing.NewDag(config.Dag)
	rs := urn.NewUrn(config.Create.Pid)
	rs.Init(dag)
	defer dag.Stop()

	syncService, callback, err := sync.NewService(dag, rs, config.Sync, attemptTimingRequests, log)
	if err != nil {
		return nil, err
	}

	service, err := create.NewService(dag, rs, config.Create, callback, dagFinished, attemptTimingRequests, txChan, log.With().Int(logging.Service, logging.CreateService).Logger())
	if err != nil {
		return nil, err
	}
	services = append(services, service)

	service, err = order.NewService(dag, rs, config.Order, attemptTimingRequests, orderedUnits, log.With().Int(logging.Service, logging.OrderService).Logger())
	if err != nil {
		return nil, err
	}
	services = append(services, service)

	service, err = validate.NewService(dag, config.TxValidate, orderedUnits, log.With().Int(logging.Service, logging.ValidateService).Logger())
	if err != nil {
		return nil, err
	}
	services = append(services, service)

	service, err = generate.NewService(dag, config.TxGenerate, txChan, log.With().Int(logging.Service, logging.GenerateService).Logger())
	if err != nil {
		return nil, err
	}
	services = append(services, service)

	service, err = logging.NewService(config.MemLog, log.With().Int(logging.Service, logging.MemLogService).Logger())
	if err != nil {
		return nil, err
	}
	services = append(services, service)
	services = append(services, syncService)

	err = startAll(services)
	if err != nil {
		return nil, err
	}
	defer stopAll(services)
	<-dagFinished
	return dag, nil
}
