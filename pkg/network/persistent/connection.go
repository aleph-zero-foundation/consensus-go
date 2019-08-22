package persistent

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

const (
	headerSize = 12
	bufSize    = 1 << 16
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
	if buf, ok := <-cr.ch; ok {
		return bytes.NewReader(buf).Read(b)
	}
	return 0, errors.New("Read on a closed connection")

}

// conn implements network.Connection interface and represents a "virtual connection", many of which utilize the same TCP link.
// each virtual connection has a unique id and every data sent through the common TCP link is prefixed with a 12 bytes header
// consisting of connection ID (8 bytes) and the length of data (4 bytes).
// Writes are buffered and the actual network traffic happens only on Flush (which needs to be invoked manually) or when the buffer is full.
// Reads are also buffered and they read byte slices from the channel populated by the link supervising this conn.
// Close consists of sending the lone header with data length 0. After closing the conn, calling Write or Flush returns an error, reading is
// still possible until the underlying channel is depleted.
// NOTE: Write() and Flush() are NOT thread safe!
type conn struct {
	id     uint64
	link   net.Conn
	queue  *chanReader
	reader *bufio.Reader
	frame  []byte
	buffer []byte
	bufLen int
	sent   uint32
	recv   uint32
	closed int32
	log    zerolog.Logger
}

//newConn creates a Connection with given id that wraps a tcp connection link
func newConn(id uint64, link net.Conn, log zerolog.Logger) *conn {
	frame := make([]byte, headerSize+bufSize)
	binary.LittleEndian.PutUint64(frame, id)
	queue := newChanReader(32)
	return &conn{
		id:     id,
		link:   link,
		queue:  queue,
		reader: bufio.NewReaderSize(queue, bufSize),
		frame:  frame,
		buffer: frame[headerSize:],
		log:    log,
	}
}

func (c *conn) Read(b []byte) (int, error) {
	n, err := c.reader.Read(b)
	c.recv += uint32(n)
	return n, err
}

func (c *conn) Write(b []byte) (int, error) {
	if atomic.LoadInt32(&c.closed) > 0 {
		return 0, errors.New("Write on a closed connection")
	}
	total := 0
	copied := copy(c.buffer[c.bufLen:], b)
	c.bufLen += copied
	total += copied
	for total < len(b) {
		err := c.Flush()
		if err != nil {
			return total, err
		}
		copied = copy(c.buffer[c.bufLen:], b[total:])
		c.bufLen += copied
		total += copied
	}
	return total, nil
}

func (c *conn) Flush() error {
	if atomic.LoadInt32(&c.closed) > 0 {
		return errors.New("Flush on a closed connection")
	}
	if c.bufLen == 0 {
		return nil
	}
	binary.LittleEndian.PutUint32(c.frame[8:], uint32(c.bufLen))
	_, err := c.link.Write(c.frame[:(headerSize + c.bufLen)])
	if err != nil {
		return err
	}
	c.sent += uint32(c.bufLen)
	c.bufLen = 0
	return nil
}

func (c *conn) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}
	header := make([]byte, headerSize)
	binary.LittleEndian.PutUint64(header, c.id)
	binary.LittleEndian.PutUint32(header[8:], 0)
	_, err := c.link.Write(header)
	if err != nil {
		return err
	}
	c.finalize()
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

func (c *conn) enqueue(b []byte) {
	if atomic.LoadInt32(&c.closed) == 0 {
		c.queue.ch <- b
	}
}

func (c *conn) localClose() {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		c.finalize()
	}
}

func (c *conn) finalize() {
	close(c.queue.ch)
	c.log.Info().Uint32(logging.Sent, c.sent).Uint32(logging.Recv, c.recv).Uint64(logging.ID, c.id).Msg(logging.ConnectionClosed)
}
