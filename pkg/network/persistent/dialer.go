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
	link       net.Conn
	conns      map[uint64]*conn
	remoteAddr string
	timeout    time.Duration
	lastID     uint64
	mx         sync.Mutex
	wg         *sync.WaitGroup
	quit       *int32
	log        zerolog.Logger
}

func newDialer(remoteAddress string, timeout time.Duration, wg *sync.WaitGroup, quit *int32, log zerolog.Logger) (*dialer, error) {
	link, err := net.DialTimeout("tcp", remoteAddress, timeout)
	if err != nil {
		return nil, err
	}
	return &dialer{
		link:       link,
		conns:      make(map[uint64]*conn),
		remoteAddr: remoteAddress,
		timeout:    timeout,
		wg:         wg,
		quit:       quit,
		log:        log,
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
				d.reconnect()
				continue
			}
			id, size := parseHeader(hdr)
			d.mx.Lock()
			conn, ok := d.conns[id]
			d.mx.Unlock()
			if ok && size == 0 {
				conn.Close()
				continue
			}
			buf := make([]byte, size)
			_, err = io.ReadFull(d.link, buf)
			if err != nil {
				d.log.Error().Str("where", "persistent.dialer.body").Msg(err.Error())
				d.reconnect()
				continue
			}
			if ok {
				conn.append(buf)
			} else {
				d.log.Error().Str("where", "persistent.dialer").Msg("incorrect conn ID")
			}
		}
	}()
}

func (d *dialer) reconnect() {
	d.stop()
	link, err := net.DialTimeout("tcp", d.remoteAddr, d.timeout)
	if err != nil {
		d.log.Error().Str("where", "persistent.dialer.reconnect").Msg(err.Error())
		return
	}
	d.mx.Lock()
	defer d.mx.Unlock()
	d.link = link
	d.conns = make(map[uint64]*conn)
	d.lastID = 0
}

func (d *dialer) stop() {
	d.mx.Lock()
	d.link.Close()
	defer d.mx.Unlock()
	for _, conn := range d.conns {
		conn.Close()
	}
}

func (d *dialer) dial() network.Connection {
	d.mx.Lock()
	defer d.mx.Unlock()
	conn := newConn(d.lastID, d.link, d.log)
	d.conns[d.lastID] = conn
	d.lastID++
	return conn
}
