package forking

import (
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/core-go/pkg/network"
	"gitlab.com/alephledger/core-go/pkg/rmcbox"
)

type service struct {
	*alertHandler
	netserv network.Server
	listens sync.WaitGroup
	quit    int64
	log     zerolog.Logger
}

// NewAlerter constructs an alerting service for the given dag with the given configuration.
func NewAlerter(conf config.Config, orderer gomel.Orderer, netserv network.Server, log zerolog.Logger) (gomel.Alerter, error) {
	rmc := rmcbox.New(conf.RMCPublicKeys, conf.RMCPrivateKey)
	a := newAlertHandler(conf, orderer, rmc, netserv, log)
	s := &service{
		alertHandler: a,
		netserv:      netserv,
		log:          log.With().Int(logging.Service, logging.AlertService).Logger(),
	}
	return s, nil
}

func (s *service) Start() {
	s.listens.Add(1)
	go s.handleConns()
	s.log.Log().Msg(logging.ServiceStarted)
}

func (s *service) Stop() {
	atomic.StoreInt64(&s.quit, 1)
	s.listens.Wait()
	s.log.Log().Msg(logging.ServiceStopped)
}

func (s *service) handleConns() {
	defer s.listens.Done()
	for atomic.LoadInt64(&s.quit) == 0 {
		conn, err := s.netserv.Listen()
		if err != nil {
			continue
		}
		s.listens.Add(1)
		go func() {
			defer s.listens.Done()
			s.HandleIncoming(conn)
		}()
	}
}
