package network_test

import (
	"bufio"
	"bytes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"
	"os"
	"time"

	. "gitlab.com/alephledger/consensus-go/pkg/network"
)

var _ = Describe("Multicast", func() {

	var (
		conns []Connection
		mc    *Multicaster

		msg []byte
		N   int
	)

	Describe("using buffers", func() {
		Describe("multicasting", func() {
			BeforeEach(func() {
				log := zerolog.New(bufio.NewWriter(os.Stdout))
				N = 10
				conns = make([]Connection, N)
				for i := 0; i < N; i++ {
					conns[i] = &connection{new(bytes.Buffer), log}
				}
				mc = NewMulticaster(conns)
				msg = []byte("test")
			})
			It("Should deliver messages to all recipents", func() {
				n, err := mc.Write(msg)
				Expect(n).To(Equal(len(msg)))
				Expect(err).NotTo(HaveOccurred())
				for i := 0; i < N; i++ {
					b := make([]byte, len(msg))
					n, err = conns[i].Read(b)
					Expect(n).To(Equal(len(msg)))
					Expect(err).NotTo(HaveOccurred())
					Expect(bytes.Equal(b, msg)).To(BeTrue())
				}
			})
		})
	})
})

type connection struct {
	link *bytes.Buffer
	log  zerolog.Logger
}

func (c *connection) Read(b []byte) (int, error) {
	return c.link.Read(b)
}

func (c *connection) Write(b []byte) (int, error) {
	n, err := c.link.Write(b)
	return n, err
}

func (c *connection) Flush() error {
	return newNetError("not implemented")
}

func (c *connection) Close() error {
	if c.link == nil {
		return newNetError("already closed")
	}
	c.link = nil

	return nil
}

func (c *connection) TimeoutAfter(t time.Duration) {
}

func (c *connection) Log() zerolog.Logger {
	return c.log
}

func (c *connection) SetLogger(log zerolog.Logger) {
	c.log = log
}

type netError struct {
	msg string
}

func newNetError(msg string) *netError {
	return &netError{msg}
}

func (ne *netError) Error() string {
	return ne.msg
}
