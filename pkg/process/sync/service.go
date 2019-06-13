package sync

import (
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/semaphore"

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
func NewService(poset gomel.Poset, config *process.Sync, attemptTiming chan<- int, log zerolog.Logger) (process.Service, error) {
	dial := newDialer(poset.NProc(), config.Pid, config.SyncInitDelay)
	dialSem := semaphore.NewWeighted(int64(config.InitializedSyncLimit))
	listenSem := semaphore.NewWeighted(int64(config.ReceivedSyncLimit))
	connServ, err := tcp.NewConnServer(config.LocalAddress, config.RemoteAddresses, dial.channel(), listenSem, dialSem, uint16(config.Pid), log)
	if err != nil {
		return nil, err
	}
	requestIn := &request.In{Timeout: config.Timeout, MyPid: config.Pid, AttemptTiming: attemptTiming}
	requestOut := &request.Out{Timeout: config.Timeout, MyPid: config.Pid, AttemptTiming: attemptTiming}
	syncServ := ssync.NewServer(poset, listenSem, dialSem, connServ.ListenChannel(), connServ.DialChannel(), requestIn, requestOut, config.InitializedSyncLimit, config.ReceivedSyncLimit)
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
