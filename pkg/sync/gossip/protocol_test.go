package gossip_test

import (
	"io"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

type poset struct {
	*tests.Poset
	attemptedAdd []gomel.Preunit
}

func (p *poset) AddUnit(unit gomel.Preunit, callback func(gomel.Preunit, gomel.Unit, error)) {
	p.attemptedAdd = append(p.attemptedAdd, unit)
	p.Poset.AddUnit(unit, callback)
}

type connection struct {
	in  io.Reader
	out io.Writer
	log zerolog.Logger
}

func (c *connection) Read(buf []byte) (int, error) {
	return c.in.Read(buf)
}

func (c *connection) Write(buf []byte) (int, error) {
	return c.out.Write(buf)
}

func (c *connection) Flush() error {
	return nil
}

func (c *connection) Close() error {
	return nil
}

func (c *connection) TimeoutAfter(time.Duration) {}

func (c *connection) Log() zerolog.Logger {
	return c.log
}

func (c *connection) SetLogger(zerolog.Logger) {}

func newConnection() (network.Connection, network.Connection) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	return &connection{r1, w2, zerolog.Logger{}}, &connection{r2, w1, zerolog.Logger{}}
}

type dialer struct {
	conn network.Connection
}

// NOTE: this ignores the argument, which is not good.
// However, for the tests below it should be sufficient.
func (d *dialer) Dial(uint16) (network.Connection, error) {
	return d.conn, nil
}

func (d *dialer) DialAll() (network.Multicaster, error) {
	return nil, nil
}

// NOTE: since we return 2, we should always only ask about the other peer, which makes the above note somewhat irrelevant.
// Still only sufficient for the tests below.
func (d *dialer) Length() int {
	return 2
}

var _ = Describe("Protocol", func() {

	var (
		p1     *poset
		p2     *poset
		proto1 gsync.Protocol
		proto2 gsync.Protocol
		c      network.Connection
		d      network.Dialer
	)

	BeforeEach(func() {
		c1, c2 := newConnection()
		c = c1
		d = &dialer{c2}
	})

	JustBeforeEach(func() {
		proto1 = NewProtocol(0, p1, d, time.Second, make(chan int), zerolog.Logger{})
		proto2 = NewProtocol(1, p2, d, time.Second, make(chan int), zerolog.Logger{})
	})

	Describe("in a small poset", func() {

		Context("when both copies are empty", func() {

			BeforeEach(func() {
				tp1, _ := tests.CreatePosetFromTestFile("../../testdata/empty.txt", tests.NewTestPosetFactory())
				p1 = &poset{
					Poset:        tp1.(*tests.Poset),
					attemptedAdd: nil,
				}
				tp2, _ := tests.CreatePosetFromTestFile("../../testdata/empty.txt", tests.NewTestPosetFactory())
				p2 = &poset{
					Poset:        tp2.(*tests.Poset),
					attemptedAdd: nil,
				}
			})

			It("should not add anything", func() {
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					proto1.In(c)
					wg.Done()
				}()
				go func() {
					proto2.Out()
					wg.Done()
				}()
				wg.Wait()
				Expect(p1.attemptedAdd).To(BeEmpty())
				Expect(p2.attemptedAdd).To(BeEmpty())
			})
		})

		Context("when the first copy contains a single dealing unit", func() {

			BeforeEach(func() {
				tp1, _ := tests.CreatePosetFromTestFile("../../testdata/one_unit.txt", tests.NewTestPosetFactory())
				p1 = &poset{
					Poset:        tp1.(*tests.Poset),
					attemptedAdd: nil,
				}
				tp2, _ := tests.CreatePosetFromTestFile("../../testdata/empty.txt", tests.NewTestPosetFactory())
				p2 = &poset{
					Poset:        tp2.(*tests.Poset),
					attemptedAdd: nil,
				}
			})

			It("should add the unit to the second copy", func() {
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					proto1.In(c)
					wg.Done()
				}()
				go func() {
					proto2.Out()
					wg.Done()
				}()
				wg.Wait()
				Expect(p1.attemptedAdd).To(BeEmpty())
				Expect(p2.attemptedAdd).To(HaveLen(1))
				Expect(p2.attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(p2.attemptedAdd[0].Creator()).To(BeNumerically("==", 0))
			})

		})

		Context("when the second copy contains a single dealing unit", func() {

			BeforeEach(func() {
				tp1, _ := tests.CreatePosetFromTestFile("../../testdata/empty.txt", tests.NewTestPosetFactory())
				p1 = &poset{
					Poset:        tp1.(*tests.Poset),
					attemptedAdd: nil,
				}
				tp2, _ := tests.CreatePosetFromTestFile("../../testdata/other_unit.txt", tests.NewTestPosetFactory())
				p2 = &poset{
					Poset:        tp2.(*tests.Poset),
					attemptedAdd: nil,
				}
			})

			It("should add the unit to the first copy", func() {
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					proto1.In(c)
					wg.Done()
				}()
				go func() {
					proto2.Out()
					wg.Done()
				}()
				wg.Wait()
				Expect(p2.attemptedAdd).To(BeEmpty())
				Expect(p1.attemptedAdd).To(HaveLen(1))
				Expect(p1.attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(p1.attemptedAdd[0].Creator()).To(BeNumerically("==", 1))
			})

		})

		Context("when both copies contain all the dealing units", func() {

			BeforeEach(func() {
				tp1, _ := tests.CreatePosetFromTestFile("../../testdata/only_dealing.txt", tests.NewTestPosetFactory())
				p1 = &poset{
					Poset:        tp1.(*tests.Poset),
					attemptedAdd: nil,
				}
				tp2 := tp1
				p2 = &poset{
					Poset:        tp2.(*tests.Poset),
					attemptedAdd: nil,
				}
			})

			It("should not add anything", func() {
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					proto1.In(c)
					wg.Done()
				}()
				go func() {
					proto2.Out()
					wg.Done()
				}()
				wg.Wait()
				Expect(p1.attemptedAdd).To(BeEmpty())
				Expect(p2.attemptedAdd).To(BeEmpty())
			})

		})

	})

})
