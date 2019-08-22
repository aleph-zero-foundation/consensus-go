package persistent

import (
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

// link wraps a persistent TCP connection and distributes incoming traffic to multiple virtual connections (conn).
// It comes in two variants: outgoing link (allows creating new conns with call()) and incoming link (creates a new conn
// upon receiving data with unknown id and puts that conn on the listener queue). Two variants are distinguished by the
// existence of that queue: outgoing link has nil as queue, incoming a non-nil channel.
// When encountering an error during reading, the link shuts down the TCP connection and marks itself "dead". To restore
// the communication new link needs to be created.
type link struct {
	tcpLink net.Conn
	conns   map[uint64]*conn
	queue   chan network.Connection
	lastID  uint64
	mx      sync.Mutex
	wg      *sync.WaitGroup
	quit    *int32
	log     zerolog.Logger
}

func newLink(tcpLink net.Conn, queue chan network.Connection, wg *sync.WaitGroup, quit *int32, log zerolog.Logger) *link {
	return &link{
		tcpLink: tcpLink,
		conns:   make(map[uint64]*conn),
		queue:   queue,
		wg:      wg,
		quit:    quit,
		log:     log,
	}
}

func (l *link) start() {
	go func() {
		l.wg.Add(1)
		defer l.wg.Done()
		hdr := make([]byte, headerSize)
		for atomic.LoadInt32(l.quit) == 0 {
			_, err := io.ReadFull(l.tcpLink, hdr)
			if err != nil {
				l.log.Error().Str("where", "persistent.link.header").Msg(err.Error())
				l.stop()
				return
			}
			id, size := parseHeader(hdr)
			conn, ok := l.getConn(id)
			if ok && size == 0 {
				conn.localClose()
				continue
			}
			buf := make([]byte, size)
			_, err = io.ReadFull(l.tcpLink, buf)
			if err != nil {
				l.log.Error().Str("where", "persistent.link.body").Msg(err.Error())
				l.stop()
				return
			}
			if ok {
				conn.enqueue(buf)
				continue
			}
			if l.isOut() {
				l.log.Error().Uint64(logging.ID, id).Str("where", "persistent.link").Msg("incorrect conn ID")
			} else {
				nc := newConn(id, l.tcpLink, l.log)
				nc.enqueue(buf)
				l.addConn(nc)
				l.queue <- nc
			}
		}
	}()
}

func (l *link) getConn(id uint64) (*conn, bool) {
	l.mx.Lock()
	defer l.mx.Unlock()
	conn, ok := l.conns[id]
	return conn, ok
}

func (l *link) addConn(c *conn) {
	l.mx.Lock()
	defer l.mx.Unlock()
	l.conns[c.id] = c
}

func (l *link) isOut() bool {
	return l.queue == nil
}

func (l *link) isDead() bool {
	l.mx.Lock()
	defer l.mx.Unlock()
	return l.tcpLink == nil
}

func (l *link) stop() {
	l.mx.Lock()
	defer l.mx.Unlock()
	l.tcpLink.Close()
	l.tcpLink = nil
	for _, conn := range l.conns {
		conn.Close()
	}
}

func (l *link) call() network.Connection {
	if !l.isOut() {
		l.log.Error().Str("where", "persistent.link.call").Msg("call() called on inbound link")
		return nil
	}
	l.mx.Lock()
	defer l.mx.Unlock()
	conn := newConn(l.lastID, l.tcpLink, l.log)
	l.conns[l.lastID] = conn
	l.lastID++
	return conn
}
