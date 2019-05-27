package sync

import (
	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	ssync "gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/request"
)

type service struct {
	syncServer *ssync.Server
	connServer network.ConnectionServer
	dialer     *dialer
	log        zerolog.Logger
}

// NewService creates a new syncing service for the given poset, with the given config.
func NewService(poset gomel.Poset, config *process.Sync, log zerolog.Logger) (process.Service, error) {
	dial := newDialer(poset.NProc(), config.SyncInitDelay)
	connServ, err := tcp.NewConnServer(config.LocalAddress, config.RemoteAddresses, dial.channel(), config.ListenQueueLength, config.SyncQueueLength)
	if err != nil {
		return nil, err
	}
	syncServ := ssync.NewServer(poset, connServ.ListenChannel(), connServ.DialChannel(), request.In{Timeout: config.Timeout}, request.Out{Timeout: config.Timeout}, config.InitializedSyncLimit, config.ReceivedSyncLimit)
	return &service{
		syncServer: syncServ,
		connServer: connServ,
		dialer:     dial,
		log:        log,
	}, nil
}

func (s *service) Start() error {
	err := s.connServer.Listen()
	if err != nil {
		return err
	}
	s.syncServer.Start()
	s.connServer.StartDialing()
	s.dialer.start()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	s.dialer.stop()
	s.connServer.Stop()
	s.syncServer.Stop()
	s.log.Info().Msg(logging.ServiceStopped)
}
