package run

import (
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
	var services []process.Service
	poset := growing.NewPoset(config.Keys)
	defer poset.Stop()
	service, err := create.NewService(poset, config.Create, done)
	if err != nil {
		return err
	}
	services = append(services, service)
	service, err = sync.NewService(poset, config.Sync)
	if err != nil {
		return err
	}
	services = append(services, service)
	service, err = order.NewService(poset, config.Order)
	if err != nil {
		return err
	}
	services = append(services, service)
	service, err = validate.NewService(poset, config.Validate)
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
