package generate

import (
	"bufio"
	"math/rand"
	"os"

	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

type service struct {
	users    []string
	txChan   chan<- *gomel.Tx
	exitChan chan struct{}
	log      zerolog.Logger
}

func readUsers(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	users := []string{}
	for scanner.Scan() {
		users = append(users, scanner.Text())
	}
	return users, nil
}

// NewService creates a new service generating transactions
func NewService(poset gomel.Poset, config *process.TxGenerate, txChan chan<- *gomel.Tx, log zerolog.Logger) (process.Service, error) {
	users, err := readUsers(config.UserDb)
	if err != nil {
		return nil, err
	}
	return &service{
		users:    users,
		txChan:   txChan,
		exitChan: make(chan struct{}),
		log:      log,
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
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	close(s.exitChan)
	s.log.Info().Msg(logging.ServiceStopped)
}
