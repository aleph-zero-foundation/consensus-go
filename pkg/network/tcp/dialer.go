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
	link, err := net.DialTimeout("tcp", d.remoteAddrs[pid], time.Second*2)
	if err != nil {
		return nil, err
	}
	return newConn(link, d.log), nil
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
