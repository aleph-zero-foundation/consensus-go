package fallback_test

import (
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/fallback"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Retrying", func() {

	var (
		p        gomel.Poset
		ps       []gomel.Poset
		reqs     chan fetch.Request
		fallback *Retrying
		proto    gsync.Protocol
		protos   []gsync.Protocol
		d        *tests.Dialer
		ls       []network.Listener
		interval time.Duration
	)

	BeforeEach(func() {
		d, ls = tests.NewNetwork(10)
		reqs = make(chan fetch.Request, 100)
	})

	JustBeforeEach(func() {
		baseFallback := NewFetch(p, reqs)
		rs1 := tests.NewTestRandomSource(p)
		fallback = NewRetrying(baseFallback, p, rs1, interval, zerolog.Nop())
		fallback.Start()
		proto = fetch.NewProtocol(0, p, rs1, reqs, d, ls[0], time.Second, fallback, make(chan int), zerolog.Nop())
		for i, op := range ps {
			protos = append(protos, fetch.NewProtocol(uint16(i+1), op, tests.NewTestRandomSource(op), reqs, d, ls[i+1], time.Second, nil, make(chan int), zerolog.Nop()))
		}
	})

	JustAfterEach(func() {
		fallback.Stop()
	})

	Describe("when we did not make any units", func() {

		var (
			callee uint16
		)

		BeforeEach(func() {
			callee = 1
		})

		Context("when requesting a unit with unknown parents", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				interval = 10 * time.Millisecond
				p, _ = tests.CreatePosetFromTestFile("../../testdata/empty.txt", tests.NewTestPosetFactory())
				op, _ := tests.CreatePosetFromTestFile("../../testdata/random_10p_100u_2par_dead0.txt", tests.NewTestPosetFactory())
				for range ls {
					ps = append(ps, op)
				}
				ps = ps[1:]
				maxes := op.MaximalUnitsPerProcess()
				// Pick the hash of any maximal unit.
				maxes.Iterate(func(units []gomel.Unit) bool {
					for _, u := range units {
						theUnit = u
						return false
					}
					return true
				})
			})

			It("should eventually add the unit", func(done Done) {
				var wg sync.WaitGroup
				var quit int32
				for _, prot := range protos {
					wg.Add(1)
					go func(p gsync.Protocol) {
						defer wg.Done()
						for atomic.LoadInt32(&quit) != 1 {
							p.In()
						}
					}(prot)
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					for atomic.LoadInt32(&quit) != 1 {
						proto.Out()
					}
				}()
				req := fetch.Request{
					Pid:    callee,
					Hashes: []*gomel.Hash{theUnit.Hash()},
				}
				reqs <- req
				added := false
				uh := []*gomel.Hash{theUnit.Hash()}
				for !added {
					time.Sleep(4 * interval)
					theUnitTransferred := p.Get(uh)[0]
					if theUnitTransferred != nil {
						added = true
						Expect(theUnitTransferred.Creator()).To(Equal(theUnit.Creator()))
						Expect(theUnitTransferred.Signature()).To(Equal(theUnit.Signature()))
						Expect(theUnitTransferred.Data()).To(Equal(theUnit.Data()))
						Expect(theUnitTransferred.RandomSourceData()).To(Equal(theUnit.RandomSourceData()))
						Expect(theUnitTransferred.Hash()).To(Equal(theUnit.Hash()))
					}
				}
				atomic.StoreInt32(&quit, 1)
				close(reqs)
				d.Close()
				wg.Wait()
				close(done)
			}, 30)

		})

	})

})
