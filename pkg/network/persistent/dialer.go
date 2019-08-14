package persistent

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type dialer struct {
	link   net.Conn
	conns  map[uint64]*conn
	lastID uint64
	mx     sync.Mutex
	wg     *sync.WaitGroup
	quit   *int32
	log    zerolog.Logger
}

func newDialer(remoteAddress string, timeout time.Duration, wg *sync.WaitGroup, quit *int32, log zerolog.Logger) (*dialer, error) {
	link, err := net.DialTimeout("tcp", remoteAddress, timeout)
	if err != nil {
		return nil, err
	}
	return &dialer{
		link:  link,
		conns: make(map[uint64]*conn),
		wg:    wg,
		quit:  quit,
		log:   log,
	}, nil
}

func (d *dialer) start() {
	go func() {
		d.wg.Add(1)
		defer d.wg.Done()
		hdr := make([]byte, headerSize)
		for {
			if atomic.LoadInt32(d.quit) > 0 {
				return
			}
			_, err := io.ReadFull(d.link, hdr)
			if err != nil {
				d.log.Error().Str("where", "persistent.dialer.header").Msg(err.Error())
				return
			}
			id, size := parseHeader(hdr)
			buf := make([]byte, size)
			_, err = io.ReadFull(d.link, buf)
			if err != nil {
				d.log.Error().Str("where", "persistent.dialer.body").Msg(err.Error())
				return
			}
			if conn, ok := d.conns[id]; ok {
				conn.append(buf)
			} else {
				d.log.Error().Str("where", "persistent.dialer").Msg("incorrect connection ID")
			}

		}
	}()
}

func (d *dialer) stop() {
	d.link.Close()
}

func (d *dialer) dial() network.Connection {
	d.mx.Lock()
	defer d.mx.Unlock()
	conn := newConn(d.lastID, d.link, d.log)
	d.conns[d.lastID] = newConn(d.lastID, d.link, d.log)
	d.lastID++
	return conn
}
