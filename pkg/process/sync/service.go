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
	"gitlab.com/alephledger/consensus-go/pkg/sync/request"
)

type service struct {
	syncServer *gsync.Server
	connServer network.ConnectionServer
	dialer     *dialer
	log        zerolog.Logger
}

// NewService creates a new syncing service for the given poset, with the given config.
func NewService(poset gomel.Poset, config *process.Sync, attemptTiming chan<- int, log zerolog.Logger) (process.Service, error) {
	dial := newDialer(poset.NProc(), config.Pid, config.SyncInitDelay)
	inUse := make([]*gsync.Mutex, len(config.RemoteAddresses))
	for i := range inUse {
		inUse[i] = gsync.NewMutex()
	}
	connServ, err := tcp.NewConnServer(config.LocalAddress, config.RemoteAddresses, dial.channel(), inUse, log)
	if err != nil {
		return nil, err
	}
	requestIn := &request.In{Timeout: config.Timeout, MyPid: config.Pid, AttemptTiming: attemptTiming}
	requestOut := &request.Out{Timeout: config.Timeout, MyPid: config.Pid, AttemptTiming: attemptTiming}
	syncServ := gsync.NewServer(uint16(config.Pid), poset, connServ.ListenChannel(), connServ.DialChannel(), requestIn, requestOut, config.InitializedSyncLimit, config.ReceivedSyncLimit, config.RemoteAddresses, inUse, log)
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
	// let other processes sync with us some more
	time.Sleep(10 * time.Second)
	s.connServer.Stop()
	s.syncServer.Stop()
	s.log.Info().Msg(logging.ServiceStopped)
}
