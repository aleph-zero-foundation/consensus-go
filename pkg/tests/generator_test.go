package tests_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	. "gitlab.com/alephledger/consensus-go/pkg/tests"
)

func countUnits(dag gomel.Dag) int {
	seenUnits := make(map[gomel.Hash]bool)
	queue := []gomel.Unit{}
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			queue = append(queue, u)
			seenUnits[*u.Hash()] = true
		}
		return true
	})
	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]
		seenUnits[*w.Hash()] = true
		for _, wParent := range w.Parents() {
			if wParent == nil {
				continue
			}
			if !seenUnits[*wParent.Hash()] {
				queue = append(queue, wParent)
				seenUnits[*wParent.Hash()] = true
			}
		}
	}
	return len(seenUnits)
}

var _ = Describe("Generator", func() {
	Describe("CreateRandomNonForking", func() {
		var dag gomel.Dag
		Context("Called with nProcesses = 10, nUnits = 50", func() {
			dag = CreateRandomNonForking(10, 50)
			It("Should return dag with 10 processes", func() {
				Expect(dag.NProc()).To(Equal(uint16(10)))
			})
			It("Should have 50 units", func() {
				Expect(countUnits(dag)).To(Equal(50))
			})
		})
	})
})
