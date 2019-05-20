package generate

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"math/rand"
	"time"
)

type service struct {
	users    []string
	ticker   *time.Ticker
	txChan   chan<- *gomel.Tx
	exitChan chan struct{}
}

// NewService creates a new service generating transactions
func NewService(poset gomel.Poset, config *process.Generate, txChan chan<- *gomel.Tx) (process.Service, error) {
	return &service{
		users:    config.Users,
		ticker:   time.NewTicker(time.Duration(config.Frequency) * time.Millisecond),
		txChan:   txChan,
		exitChan: make(chan struct{}),
	}, nil
}

func (s *service) main() {
	var txID uint32
	for {
		select {
		case <-s.ticker.C:
			s.txChan <- &gomel.Tx{
				ID:       txID,
				Receiver: s.users[rand.Intn(len(s.users))],
				Issuer:   s.users[rand.Intn(len(s.users))],
				Amount:   uint32(rand.Intn(10)),
			}
		case <-s.exitChan:
			s.ticker.Stop()
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
