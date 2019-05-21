package generate

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"math/rand"
)

type service struct {
	users    []string
	txChan   chan<- *gomel.Tx
	exitChan chan struct{}
}

// NewService creates a new service generating transactions
func NewService(poset gomel.Poset, config *process.Generate, txChan chan<- *gomel.Tx) (process.Service, error) {
	return &service{
		users:    config.Users,
		txChan:   txChan,
		exitChan: make(chan struct{}),
	}, nil
}

func (s *service) main() {
	var txID uint32
	for {
		select {
		case s.txChan <- &gomel.Tx{
			ID:       txID,
			Receiver: s.users[rand.Intn(len(s.users))],
			Issuer:   s.users[rand.Intn(len(s.users))],
			Amount:   uint32(rand.Intn(10)),
		}:
			txID++
		case <-s.exitChan:
			close(s.txChan)
			return
		}
	}
}

func (s *service) Start() error {
	go s.main()
	return nil
}

func (s *service) Stop() {
	close(s.exitChan)
}
