package persistent

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

const (
	headerSize = 12
	bufSize    = 2 << 15
)

func parseHeader(header []byte) (uint64, uint32) {
	return binary.LittleEndian.Uint64(header), binary.LittleEndian.Uint32(header[8:])
}

type chanReader struct {
	ch chan []byte
}

func newChanReader(size int) *chanReader {
	return &chanReader{ch: make(chan []byte, size)}
}

func (cr *chanReader) Read(b []byte) (int, error) {
	return bytes.NewReader(<-cr.ch).Read(b)
}

type conn struct {
	id     uint64
	link   net.Conn
	queue  *chanReader
	reader *bufio.Reader
	writer *bufio.Writer
	header []byte
	sent   uint32
	recv   uint32
	mx     sync.Mutex
	log    zerolog.Logger
}

//newConn creates a Connection with given id that wraps a tcp connection link
func newConn(id uint64, link net.Conn, log zerolog.Logger) *conn {
	header := make([]byte, headerSize)
	binary.LittleEndian.PutUint64(header, id)
	queue := newChanReader(32)
	return &conn{
		id:     id,
		link:   link,
		queue:  queue,
		reader: bufio.NewReaderSize(queue, bufSize),
		writer: bufio.NewWriterSize(link, bufSize),
		header: header,
		log:    log,
	}
}

func (c *conn) Read(b []byte) (int, error) {
	n, err := c.reader.Read(b)
	c.recv += uint32(n)
	return n, err
}

func (c *conn) Write(b []byte) (int, error) {
	written, n := 0, 0
	var err error
	for written < len(b) {
		n, err = c.writer.Write(b[written:])
		written += n
		if err == bufio.ErrBufferFull {
			err = c.Flush()
		}
		if err != nil {
			break
		}
	}
	c.sent += uint32(written)
	return written, err
}

func (c *conn) Flush() error {
	binary.LittleEndian.PutUint32(c.header[8:], uint32(c.writer.Buffered()))
	c.mx.Lock()
	defer c.mx.Unlock()
	_, err := c.link.Write(c.header)
	if err != nil {
		return err
	}
	err = c.writer.Flush()
	if err != nil {
		return err
	}
	return nil
}

func (c *conn) Close() error {
	// T0D0: remove from conns map, maybe send some special signal
	c.log.Info().Uint32(logging.Sent, c.sent).Uint32(logging.Recv, c.recv).Uint64(logging.ID, c.id).Msg(logging.ConnectionClosed)
	return nil
}

func (c *conn) TimeoutAfter(t time.Duration) {
	go func() {
		time.Sleep(t)
		c.Close()
	}()
}

func (c *conn) Log() zerolog.Logger {
	return c.log
}

func (c *conn) SetLogger(log zerolog.Logger) {
	c.log = log
}

func (c *conn) append(b []byte) {
	c.queue.ch <- b
}
