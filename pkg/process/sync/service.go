package sync

import (
	"time"

	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
)

type service struct {
	syncServer *gsync.Server
	connServer network.ConnectionServer
	log        zerolog.Logger
}

// NewService creates a new syncing service for the given poset, with the given config.
func NewService(poset gomel.Poset, randomSource gomel.RandomSource, config *process.Sync, attemptTiming chan<- int, log zerolog.Logger) (process.Service, error) {
	listenChan := make(chan network.Connection)
	connServ, err := tcp.NewConnServer(config.LocalAddress, listenChan, log)
	if err != nil {
		return nil, err
	}
	proto := gossip.NewProtocol(uint16(config.Pid), poset, randomSource, tcp.NewDialer(config.RemoteAddresses, log), config.Timeout, attemptTiming, log)
	syncServ := gsync.NewServer(listenChan, proto, config.OutSyncLimit, config.InSyncLimit, log)
	return &service{
		syncServer: syncServ,
		connServer: connServ,
		log:        log,
	}, nil
}

func (s *service) Start() error {
	err := s.connServer.Start()
	if err != nil {
		return err
	}
	s.syncServer.Start()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	s.syncServer.StopOut()
	// let other processes sync with us some more
	time.Sleep(time.Second)
	s.connServer.Stop()
	s.syncServer.StopIn()
	s.log.Info().Msg(logging.ServiceStopped)
}
