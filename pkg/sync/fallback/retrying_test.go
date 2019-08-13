package fallback_test

import (
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/fallback"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Retrying", func() {

	var (
		dag      gomel.Dag
		dags     []gomel.Dag
		reqs     chan fetch.Request
		fallback *Retrying
		proto    gsync.Protocol
		protos   []gsync.Protocol
		servs    []network.Server
		interval time.Duration
	)

	BeforeEach(func() {
		servs = tests.NewNetwork(10)
		reqs = make(chan fetch.Request, 100)
	})

	JustBeforeEach(func() {
		baseFallback := NewFetch(dag, reqs)
		rs1 := tests.NewTestRandomSource()
		rs1.Init(dag)
		fallback = NewRetrying(baseFallback, dag, rs1, interval, zerolog.Nop())
		fallback.Start()
		proto = fetch.NewProtocol(0, dag, rs1, reqs, servs[0], gomel.NopCallback, time.Second, fallback, zerolog.Nop())
		for i, op := range dags {
			trs := tests.NewTestRandomSource()
			trs.Init(op)
			protos = append(protos, fetch.NewProtocol(uint16(i+1), op, trs, reqs, servs[i+1], gomel.NopCallback, time.Second, nil, zerolog.Nop()))
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
				dag, _ = tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				op, _ := tests.CreateDagFromTestFile("../../testdata/random_10p_100u_2par_dead0.txt", tests.NewTestDagFactory())
				for range servs {
					dags = append(dags, op)
				}
				dags = dags[1:]
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
					theUnitTransferred := dag.Get(uh)[0]
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
				tests.CloseNetwork(servs)
				wg.Wait()
				close(done)
			}, 30)

		})

	})

})
