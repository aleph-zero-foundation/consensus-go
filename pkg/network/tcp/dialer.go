package tcp

import (
	"net"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type dialer struct {
	remoteAddrs []string
	log         zerolog.Logger
}

//NewDialer creates a dialer for the given addresses.
func NewDialer(remoteAddrs []string, log zerolog.Logger) network.Dialer {
	return &dialer{
		remoteAddrs: remoteAddrs,
		log:         log,
	}
}

func (d *dialer) Dial(pid uint16) (network.Connection, error) {
	dialer := &net.Dialer{Deadline: time.Now().Add(time.Second * 2)}
	link, err := dialer.Dial("tcp", d.remoteAddrs[pid])
	if err != nil {
		return nil, err
	}
	return NewConn(link, 0, 0, d.log), nil
}

func (d *dialer) DialAll() (*network.Multicaster, error) {
	tcpConns := make([]network.Connection, 0, len(d.remoteAddrs))
	for pid := range d.remoteAddrs {
		conn, err := d.Dial(uint16(pid))
		if err != nil {
			return nil, err
		}
		tcpConns = append(tcpConns, conn)
	}
	return network.NewMulticaster(tcpConns), nil
}

func (d *dialer) Length() int {
	return len(d.remoteAddrs)
}
