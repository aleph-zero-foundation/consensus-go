package udp

import (
	"io"
	"net"
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type dialer struct {
	remoteAddrs []string
}

// NewDialer creates a dialer for the given addresses.
func NewDialer(remoteAddrs []string) network.Dialer {
	return &dialer{
		remoteAddrs: remoteAddrs,
	}
}

func (d *dialer) Dial(pid uint16) (net.Conn, error) {
	dialer := &net.Dialer{Deadline: time.Now().Add(time.Second * 2)}
	return dialer.Dial("udp", d.remoteAddrs[pid])
}

func (d *dialer) DialAll() io.Writer {
	// TODO: implement
	return nil
}

func (d *dialer) Length() int {
	return len(d.remoteAddrs)
}
