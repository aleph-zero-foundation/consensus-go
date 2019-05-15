package sync

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	s "gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/request"
)

type service struct {
	syncServer *s.Server
	connServer network.ConnectionServer
	dialer     *dialer
}

// NewService creates a new syncing service for the given poset, with the given config.
func NewService(poset gomel.Poset, config *process.Sync) (process.Service, error) {
	dial := newDialer(poset.NProc(), config.SyncInitDelay)
	connServ, err := tcp.NewConnServer(config.LocalAddress, config.RemoteAddresses, dial.channel(), config.ListenQueueLength, config.SyncQueueLength)
	if err != nil {
		return nil, err
	}
	syncServ := s.NewServer(poset, connServ.ListenChannel(), connServ.DialChannel(), request.In{}, request.Out{}, config.InitializedSyncLimit, config.ReceivedSyncLimit)
	return &service{
		syncServer: syncServ,
		connServer: connServ,
		dialer:     dial,
	}, nil
}

func (s *service) Start() error {
	err := s.connServer.Listen()
	if err != nil {
		return err
	}
	s.syncServer.Start()
	s.connServer.Dial()
	s.dialer.start()
	return nil
}

func (s *service) Stop() {
	s.dialer.stop()
	s.connServer.Stop()
	s.syncServer.Stop()
}
