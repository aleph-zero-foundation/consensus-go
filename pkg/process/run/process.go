package run

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/process/create"
	"gitlab.com/alephledger/consensus-go/pkg/process/order"
	"gitlab.com/alephledger/consensus-go/pkg/process/sync"
	"gitlab.com/alephledger/consensus-go/pkg/process/validate"
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
	var done chan struct{}
	primeUnitCreated := make(chan struct{}, 10)
	var services []process.Service
	// attemptTimingRequests is a channel shared between orderer and creator/syncer
	// creator/syncer should send a notification to the channel when a new prime unit is added to the poset
	// orderer attempts timing decision after receiving the notification
	var attemptTimingRequests chan struct{}
	// orderedUnits is a channel shared between orderer and validator
	// orderer sends ordered units to the channel
	// validator reads the units from the channel and validates transactions contained in the unit
	var orderedUnits chan gomel.Unit
	poset := growing.NewPoset(config.Poset)
	defer poset.Stop()
	service, err := create.NewService(poset, config.Create, done, primeUnitCreated)
	if err != nil {
		return err
	}
	services = append(services, service)
	service, err = sync.NewService(poset, config.Sync)
	if err != nil {
		return err
	}
	services = append(services, service)
	service, err = order.NewService(poset, config.Order, attemptTimingRequests, orderedUnits)
	if err != nil {
		return err
	}
	services = append(services, service)
	service, err = validate.NewService(poset, config.Validate, orderedUnits)
	if err != nil {
		return err
	}
	services = append(services, service)
	err = startAll(services)
	if err != nil {
		return err
	}
	defer stopAll(services)
	<-done
	return nil
}
