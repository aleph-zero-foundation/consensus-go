package tcp

import (
	"net"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type listener struct {
	ln  *net.TCPListener
	log zerolog.Logger
}

func newListener(localAddr string, log zerolog.Logger) (network.Listener, error) {
	localTCP, err := net.ResolveTCPAddr("tcp", localAddr)
	if err != nil {
		return nil, err
	}
	ln, err := net.ListenTCP("tcp", localTCP)
	if err != nil {
		return nil, err
	}
	return &listener{
		ln:  ln,
		log: log,
	}, nil
}

func (l *listener) Listen(deadline time.Duration) (network.Connection, error) {
	l.ln.SetDeadline(time.Now().Add(deadline))
	link, err := l.ln.AcceptTCP()
	if err != nil {
		l.log.Error().Str("where", "tcp.Listener.Listen").Msg(err.Error())
		return nil, err
	}
	conn := newConn(link, l.log)
	l.log.Info().Msg(logging.ConnectionReceived)
	return conn, nil
}
