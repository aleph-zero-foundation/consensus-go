package forking

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/core-go/pkg/network"
	"gitlab.com/alephledger/core-go/pkg/network/tcp"
	"gitlab.com/alephledger/core-go/pkg/rmc"
)

type service struct {
	*alertHandler
	netserv network.Server
	timeout time.Duration
	listens sync.WaitGroup
	quit    int64
	log     zerolog.Logger
}

// NewService constructs an alerting service for the given dag with the given configuration.
func NewService(conf config.Config, orderer gomel.Orderer, log zerolog.Logger) (gomel.Alerter, error) {
	rmc := rmc.New(conf.Alert.Pubs, conf.Alert.Priv)
	netserv, err := tcp.NewServer(conf.Alert.LocalAddress, conf.Alert.RemoteAddresses)
	if err != nil {
		return nil, err
	}
	a := newAlertHandler(conf, orderer, rmc, netserv, log)
	s := &service{
		alertHandler: a,
		netserv:      netserv,
		timeout:      conf.Alert.Timeout,
		log:          log,
	}
	return s, nil
}

func (s *service) Start() error {
	s.listens.Add(1)
	go s.handleConns()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	atomic.StoreInt64(&s.quit, 1)
	s.listens.Wait()
	s.log.Info().Msg(logging.ServiceStopped)
}

func (s *service) handleConns() {
	defer s.listens.Done()
	for atomic.LoadInt64(&s.quit) == 0 {
		conn, err := s.netserv.Listen(s.timeout)
		if err != nil {
			continue
		}
		conn.TimeoutAfter(s.timeout)
		s.listens.Add(1)
		go func() {
			defer s.listens.Done()
			s.HandleIncoming(conn)
		}()
	}
}
