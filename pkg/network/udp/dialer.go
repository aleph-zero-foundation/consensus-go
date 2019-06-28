package udp

import (
	"net"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type dialer struct {
	remoteAddrs []string
	log         zerolog.Logger
}

// NewDialer creates a dialer for the given addresses.
func NewDialer(remoteAddrs []string, log zerolog.Logger) *dialer {
	return &dialer{
		remoteAddrs: remoteAddrs,
		log:         log,
	}
}

func (d *dialer) Dial(pid uint16) (network.Connection, error) {
	// can consider setting a timeout here, yet DialUDP is non-blocking, so there should be no need
	conn, err := net.Dial("udp", d.remoteAddrs[pid])
	return newOutConn(conn, d.log), err
}

func (d *dialer) DialAll() (network.Multicaster, error) {
	udpConns := make([]network.Connection, 0)
	for pid, _ := range d.remoteAddrs {
		conn, err := d.Dial(uint16(pid))
		if err != nil {
			return nil, err
		}
		udpConns = append(udpConns, conn)
	}
	return newMulticaster(udpConns), nil
}

func (d *dialer) Length() int {
	return len(d.remoteAddrs)
}
