package sync

import (
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
)

type service struct {
	gossipServer *sync.Server
	log          zerolog.Logger
}

// NewService creates a new syncing service for the given dag, with the given config.
func NewService(dag gomel.Dag, randomSource gomel.RandomSource, config *process.Sync, attemptTiming chan<- int, log zerolog.Logger) (process.Service, error) {
	listener, dialer, err := tcp.NewNetwork(config.LocalAddress, config.RemoteAddresses, log)
	if err != nil {
		return nil, err
	}
	peerSource := gossip.NewDefaultPeerSource(uint16(dag.NProc()), uint16(config.Pid))
	gossipProto := gossip.NewProtocol(uint16(config.Pid), dag, randomSource, listener, dialer, peerSource, config.Timeout, attemptTiming, log)
	gossipServ := sync.NewServer(gossipProto, config.OutSyncLimit, config.InSyncLimit, log)

	return &service{
		gossipServer: gossipServ,
		log:          log,
	}, nil
}

func (s *service) Start() error {
	s.gossipServer.Start()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	s.gossipServer.StopOut()
	// let other processes sync with us some more
	time.Sleep(time.Second)
	s.gossipServer.StopIn()
	s.log.Info().Msg(logging.ServiceStopped)
}
