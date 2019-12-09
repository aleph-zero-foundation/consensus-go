package tcp

import (
	"net"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type server struct {
	listener    *net.TCPListener
	remoteAddrs []string
	log         zerolog.Logger
}

// NewServer initializes the network setup for the given local address and the set of remote addresses.
func NewServer(localAddress string, remoteAddresses []string, log zerolog.Logger) (network.Server, error) {
	local, err := net.ResolveTCPAddr("tcp", localAddress)
	if err != nil {
		return nil, err
	}
	listener, err := net.ListenTCP("tcp", local)
	if err != nil {
		return nil, err
	}
	return &server{
		listener:    listener,
		remoteAddrs: remoteAddresses,
		log:         log,
	}, nil
}

func (s *server) Listen(timeout time.Duration) (network.Connection, error) {
	s.listener.SetDeadline(time.Now().Add(timeout))
	link, err := s.listener.Accept()
	if err != nil {
		//s.log.Error().Str("where", "tcp.server.Listen").Msg(err.Error())
		return nil, err
	}
	conn := newConn(link, s.log)
	s.log.Debug().Msg(logging.ConnectionReceived)
	return conn, nil
}

func (s *server) Dial(pid uint16, timeout time.Duration) (network.Connection, error) {
	link, err := net.DialTimeout("tcp", s.remoteAddrs[pid], timeout)
	if err != nil {
		//s.log.Error().Str("where", "tcp.server.Dial").Msg(err.Error())
		return nil, err
	}
	return newConn(link, s.log), nil
}
