package generate

import (
	"bufio"
	"math/rand"
	"os"

	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/transactions"
)

type service struct {
	users            []string
	txpu             uint32
	txChan           chan<- []byte
	exitChan         chan struct{}
	compressionLevel int
	log              zerolog.Logger
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
func NewService(poset gomel.Poset, config *process.TxGenerate, txChan chan<- []byte, log zerolog.Logger) (process.Service, error) {
	users, err := readUsers(config.UserDb)
	if err != nil {
		return nil, err
	}
	return &service{
		users:            users,
		txpu:             config.Txpu,
		txChan:           txChan,
		exitChan:         make(chan struct{}),
		compressionLevel: config.CompressionLevel,
		log:              log,
	}, nil
}

func (s *service) generateRandom(txID uint32) []transactions.Tx {
	result := []transactions.Tx{}
	for uint32(len(result)) < s.txpu {
		result = append(result, transactions.Tx{
			ID:       txID,
			Issuer:   s.users[rand.Intn(len(s.users))],
			Receiver: s.users[rand.Intn(len(s.users))],
			Amount:   uint32(rand.Intn(10)),
		})
		txID++
	}
	return result
}

func (s *service) main() {
	var txID uint32
	for {
		txs := s.generateRandom(txID)
		encodedTxs := transactions.Encode(txs)
		compressedTxs, _ := transactions.Compress(encodedTxs, s.compressionLevel)
		select {
		case s.txChan <- compressedTxs:
			txID += uint32(len(txs))
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
