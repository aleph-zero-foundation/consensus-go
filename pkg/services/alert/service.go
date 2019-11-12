// Package alert implements a service for raising alerts and using them to restrict addition to the dag.
package alert

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/forking"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
)

type service struct {
	alert   *forking.AlertHandler
	netserv network.Server
	timeout time.Duration
	listens sync.WaitGroup
	quit    int64
	log     zerolog.Logger
}

// NewService constructs an alerting service for the given dag with the given configuration.
func NewService(dag gomel.Dag, adder gomel.Adder, conf *config.Alert, log zerolog.Logger) (gomel.Service, error) {
	rmc := rmc.New(conf.Pubs, conf.Priv)
	netserv, err := tcp.NewServer(conf.LocalAddress, conf.RemoteAddresses, log)
	if err != nil {
		return nil, err
	}
	a := forking.NewAlertHandler(conf.Pid, dag, adder, conf.PublicKeys, rmc, netserv, conf.Timeout, log)
	s := &service{
		alert:   a,
		netserv: netserv,
		timeout: conf.Timeout,
		log:     log,
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
			s.log.Error().Str("where", "alertService.handleConns.Listen").Msg(err.Error())
			continue
		}
		conn.TimeoutAfter(s.timeout)
		s.listens.Add(1)
		go s.alert.HandleIncoming(conn, &s.listens)
	}
}
