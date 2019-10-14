// Package alert implements a service for raising alerts and using them to restrict addition to the dag.
package alert

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/alerter"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
)

type service struct {
	alerter *alerter.Alerter
	netserv network.Server
	timeout time.Duration
	listens sync.WaitGroup
	quit    int32
	log     zerolog.Logger
}

// NewService constructs a alerting service for the given dag with the given configuration.
func NewService(dag gomel.Dag, conf *config.Alert, log zerolog.Logger) (gomel.Dag, gomel.Service, gsync.FetchData, error) {
	rmc := rmc.New(conf.Pubs, conf.Priv)
	netserv, err := tcp.NewServer(conf.LocalAddress, conf.RemoteAddresses, log)
	if err != nil {
		return nil, nil, nil, err
	}
	a := alerter.New(conf.Pid, dag, conf.PublicKeys, rmc, netserv, conf.Timeout, log)
	return alerter.Wrap(dag, a), &service{
		alerter: a,
		netserv: netserv,
		timeout: conf.Timeout,
		log:     log,
	}, a.RequestCommitment, nil
}

func (s *service) Start() error {
	s.listens.Add(1)
	go s.handleConns()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	atomic.StoreInt32(&s.quit, 1)
	s.listens.Wait()
	s.log.Info().Msg(logging.ServiceStopped)
}

func (s *service) handleConns() {
	defer s.listens.Done()
	for atomic.LoadInt32(&s.quit) == 0 {
		conn, err := s.netserv.Listen(s.timeout)
		if err != nil {
			s.log.Error().Str("where", "alertService.handleConns.Listen").Msg(err.Error())
			continue
		}
		conn.TimeoutAfter(s.timeout)
		s.listens.Add(1)
		go s.alerter.HandleIncoming(conn, &s.listens)
	}
}
