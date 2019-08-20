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
	closed int32
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
	if atomic.LoadInt32(&c.closed) > 0 {
		return 0, errors.New("Read on a closed connection")
	}
	n, err := c.reader.Read(b)
	c.recv += uint32(n)
	return n, err
}

func (c *conn) Write(b []byte) (int, error) {
	if atomic.LoadInt32(&c.closed) > 0 {
		return 0, errors.New("Write on a closed connection")
	}
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
	if atomic.LoadInt32(&c.closed) > 0 {
		return errors.New("Flush on a closed connection")
	}
	buf := c.writer.Buffered()
	if buf == 0 {
		return nil
	}
	binary.LittleEndian.PutUint32(c.header[8:], uint32(buf))
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
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}
	header := make([]byte, headerSize)
	binary.LittleEndian.PutUint64(header, c.id)
	binary.LittleEndian.PutUint32(header[8:], uint32(0))
	_, err := c.link.Write(header)
	if err != nil {
		return err
	}
	close(c.queue.ch)
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
	if atomic.LoadInt32(&c.closed) == 0 {
		c.queue.ch <- b
	}
}
