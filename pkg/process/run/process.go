package run

import (
	"github.com/rs/zerolog/log"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
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

// Process runs all the services with the configuration provided.
// It blocks until all of them are done.
func Process(config process.Config) error {
	posetFinished := make(chan struct{})
	var services []process.Service
	// attemptTimingRequests is a channel shared between orderer and creator/syncer
	// creator/syncer should send a notification to the channel when a new prime unit is added to the poset
	// orderer attempts timing decision after receiving the notification
	attemptTimingRequests := make(chan int, 10)
	// orderedUnits is a channel shared between orderer and validator
	// orderer sends ordered units to the channel
	// validator reads the units from the channel and validates transactions contained in the unit
	// We expect to order about one level of units at once, which should be around the size of the committee.
	// The buffer has size taking that into account with some leeway.
	orderedUnits := make(chan gomel.Unit, 2*config.Poset.NProc())
	// txChan is a channel shared between tx_generator and creator
	txChan := make(chan []byte, 10)
	poset := growing.NewPoset(config.Poset)
	defer poset.Stop()

	service, err := create.NewService(poset, config.Create, posetFinished, attemptTimingRequests, txChan, log.With().Int("S", logging.CreateService).Logger())
	if err != nil {
		return err
	}
	services = append(services, service)

	service, err = sync.NewService(poset, config.Sync, log.With().Int("S", logging.SyncService).Logger())
	if err != nil {
		return err
	}
	services = append(services, service)

	service, err = order.NewService(poset, config.Order, attemptTimingRequests, orderedUnits, log.With().Int("S", logging.OrderService).Logger())
	if err != nil {
		return err
	}
	services = append(services, service)

	service, err = validate.NewService(poset, config.TxValidate, orderedUnits, log.With().Int("S", logging.ValidateService).Logger())
	if err != nil {
		return err
	}
	services = append(services, service)

	service, err = generate.NewService(poset, config.TxGenerate, txChan, log.With().Int("S", logging.GenerateService).Logger())
	if err != nil {
		return err
	}
	services = append(services, service)

	err = startAll(services)
	if err != nil {
		return err
	}
	defer stopAll(services)
	<-posetFinished
	return nil
}
