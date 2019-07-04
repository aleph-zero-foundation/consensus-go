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

func newDialer(remoteAddrs []string, log zerolog.Logger) network.Dialer {
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
	return newConn(link, d.log), nil
}

func (d *dialer) DialAll() (*network.Multicaster, error) {
	conns := make([]network.Connection, 0, len(d.remoteAddrs))
	for pid := range d.remoteAddrs {
		conn, err := d.Dial(uint16(pid))
		if err != nil {
			return nil, err
		}
		conns = append(conns, conn)
	}
	return network.NewMulticaster(conns), nil
}

func (d *dialer) Length() int {
	return len(d.remoteAddrs)
}

// NewNetwork initializes network setup for the given local address and the set of remote addresses.
// Returns a pair of complementary objects: Dialer and Listener
func NewNetwork(localAddress string, remoteAddresses []string, log zerolog.Logger) (network.Dialer, network.Listener, error) {
	listener, err := newListener(localAddress, log)
	if err != nil {
		return nil, nil, err
	}
	dialer := newDialer(remoteAddresses, log)
	return dialer, listener, nil

}
