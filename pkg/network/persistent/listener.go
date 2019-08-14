package persistent

import (
    "io"

    "net"
    "sync"
    "sync/atomic"

    "github.com/rs/zerolog"
    "gitlab.com/alephledger/consensus-go/pkg/network"
)

type listener struct {
    link  net.Conn
    conns map[uint64]*conn
    queue chan network.Connection
    wg    *sync.WaitGroup
    quit  *int32
    log   zerolog.Logger
}

func newListener(link net.Conn, queue chan network.Connection, wg *sync.WaitGroup, quit *int32, log zerolog.Logger) *listener {
    return &listener{
        link:  link,
        conns: make(map[uint64]*conn),
        queue: queue,
        wg:    wg,
        quit:  quit,
        log:   log,
    }
}

func (l *listener) start() {
    go func() {
        l.wg.Add(1)
        defer l.wg.Done()
        hdr := make([]byte, headerSize)
        for {
            if atomic.LoadInt32(l.quit) > 0 {
                return
            }
            _, err := io.ReadFull(l.link, hdr)
            if err != nil {
                l.log.Error().Str("where", "persistent.listener.header").Msg(err.Error())
                return
            }
            id, size := parseHeader(hdr)
            buf := make([]byte, size)
            _, err = io.ReadFull(l.link, buf)
            if err != nil {
                l.log.Error().Str("where", "persistent.listener.body").Msg(err.Error())
                return
            }
            if conn, ok := l.conns[id]; ok {
                conn.append(buf)
            } else {
                nc := newConn(id, l.link, l.log)
                nc.append(buf)
                l.conns[id] = nc
                l.queue <- nc
            }
        }
    }()
}
