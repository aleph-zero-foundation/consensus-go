package request_test

import (
	"io"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/request"
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
	pid uint32
	sid uint32
}

func (c *connection) Read(buf []byte) (int, error) {
	return c.in.Read(buf)
}

func (c *connection) Write(buf []byte) (int, error) {
	return c.out.Write(buf)
}

func (c *connection) Close() error {
	return nil
}

func (c *connection) TimeoutAfter(time.Duration) {}

func (c *connection) Pid() uint32 {
	return c.pid
}

func (c *connection) Sid() uint32 {
	return c.sid
}

func newConnection() (network.Connection, network.Connection) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	return &connection{r1, w2, 0, 0}, &connection{r2, w1, 0, 0}
}

var _ = Describe("Protocol", func() {

	var (
		p1  *poset
		p2  *poset
		in  gsync.Protocol
		out gsync.Protocol
		c1  network.Connection
		c2  network.Connection
	)

	BeforeEach(func() {
		in = &In{}
		out = &Out{}
		c1, c2 = newConnection()
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
					in.Run(p1, c1)
					wg.Done()
				}()
				go func() {
					out.Run(p2, c2)
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
					in.Run(p1, c1)
					wg.Done()
				}()
				go func() {
					out.Run(p2, c2)
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
					in.Run(p1, c1)
					wg.Done()
				}()
				go func() {
					out.Run(p2, c2)
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
				tp2, _ := tests.CreatePosetFromTestFile("../../testdata/only_dealing.txt", tests.NewTestPosetFactory())
				p2 = &poset{
					Poset:        tp2.(*tests.Poset),
					attemptedAdd: nil,
				}
			})

			It("should not add anything", func() {
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					in.Run(p1, c1)
					wg.Done()
				}()
				go func() {
					out.Run(p2, c2)
					wg.Done()
				}()
				wg.Wait()
				Expect(p1.attemptedAdd).To(BeEmpty())
				Expect(p2.attemptedAdd).To(BeEmpty())
			})

		})

	})

})
