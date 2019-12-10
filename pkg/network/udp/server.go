package udp

import (
	"net"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type server struct {
	listener    *net.UDPConn
	remoteAddrs []string
	log         zerolog.Logger
}

// NewServer initializes the network setup for the given local address and the set of remote addresses.
func NewServer(localAddress string, remoteAddresses []string, log zerolog.Logger) (network.Server, error) {
	local, err := net.ResolveUDPAddr("udp", localAddress)
	if err != nil {
		return nil, err
	}
	listener, err := net.ListenUDP("udp", local)
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
	buffer := make([]byte, (1 << 16))
	n, _, err := s.listener.ReadFromUDP(buffer)
	if err != nil {
		//s.log.Error().Str("where", "udp.server.Listen").Msg(err.Error())
		return nil, err
	}
	conn := newConnIn(buffer[:n], s.log)
	s.log.Debug().Msg(logging.ConnectionReceived)
	return conn, nil
}

func (s *server) Dial(pid uint16, timeout time.Duration) (network.Connection, error) {
	// can consider setting a timeout here, yet DialUDP is non-blocking, so there should be no need
	link, err := net.Dial("udp", s.remoteAddrs[pid])
	if err != nil {
		//s.log.Error().Str("where", "udp.server.Dial").Msg(err.Error())
		return nil, err
	}
	return newConnOut(link, s.log), nil
}
