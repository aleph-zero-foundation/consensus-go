package udp

import (
	"net"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type listener struct {
	ln     *net.UDPConn
	log    zerolog.Logger
	buffer []byte
}

func newListener(localAddr string, log zerolog.Logger) (network.Listener, error) {
	localUDP, err := net.ResolveUDPAddr("udp", localAddr)
	if err != nil {
		return nil, err
	}
	ln, err := net.ListenUDP("udp", localUDP)
	if err != nil {
		return nil, err
	}
	return &listener{
		ln:     ln,
		log:    log,
		buffer: make([]byte, (1 << 16)),
	}, nil
}

func (l *listener) Listen() (network.Connection, error) {
	l.ln.SetDeadline(time.Now().Add(time.Second * 2))
	n, _, err := l.ln.ReadFromUDP(l.buffer)
	if err != nil {
		l.log.Error().Str("where", "udp.Listener.Listen").Msg(err.Error())
		return nil, err
	}
	conn := NewConnIn(l.buffer[:n], l.log)
	l.log.Info().Msg(logging.ConnectionReceived)
	return conn, nil
}
