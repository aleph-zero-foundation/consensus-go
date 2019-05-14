package process_test

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	linear "gitlab.com/alephledger/consensus-go/pkg/linear"
	. "gitlab.com/alephledger/consensus-go/pkg/process"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
)

type commonRandomPermutation struct {
	n int
}

func (crp *commonRandomPermutation) Get(level int) []int {
	permutation := make([]int, crp.n, crp.n)
	for i := 0; i < crp.n; i++ {
		permutation[i] = (i + level) % crp.n
	}
	return permutation
}

func newCommonRandomPermutation(n int) *commonRandomPermutation {
	return &commonRandomPermutation{
		n: n,
	}
}

var _ = Describe("Orderer", func() {
	var (
		p                gomel.Poset
		err              error
		crp              linear.CommonRandomPermutation
		ordering         gomel.LinearOrdering
		orderingRequests chan struct{}
		statistics       chan int
		orderedUnits     chan gomel.Unit
	)
	BeforeEach(func() {
		p, err = tests.CreatePosetFromTestFile("../testdata/random_4p_100u_2par.txt", tests.NewTestPosetFactory())
		Expect(err).NotTo(HaveOccurred())
		crp = newCommonRandomPermutation(p.GetNProcesses())
		ordering = linear.NewOrdering(p, crp)
		orderingRequests = make(chan struct{})
		statistics = make(chan int)
		orderedUnits = make(chan gomel.Unit)
	})
	Context("On a fixed random poset with 4 processes and 100 units. After receving orderingRequest", func() {
		It("should write some units to result channel in order compatible with the poset order", func() {
			orderer := NewOrderer(ordering, orderingRequests, orderedUnits, statistics)
			orderer.Start()
			orderingRequests <- struct{}{}
			resultOrder := []gomel.Unit{}
			var wg sync.WaitGroup
			wg.Add(1)
			canStop := make(chan struct{})
			go func() {
				firstIt := true
				for nUnits := range statistics {
					if firstIt {
						canStop <- struct{}{}
						firstIt = false
					}
					for i := 0; i < nUnits; i++ {
						resultOrder = append(resultOrder, <-orderedUnits)
					}
				}
				wg.Done()
			}()
			<-canStop
			orderer.Stop()
			wg.Wait()
			Expect(len(resultOrder)).NotTo(Equal(0))
			for i := 0; i < len(resultOrder); i++ {
				for j := i + 1; j < len(resultOrder); j++ {
					Expect(resultOrder[i].Above(resultOrder[j])).To(BeFalse())
				}
			}
		})
	})

})
