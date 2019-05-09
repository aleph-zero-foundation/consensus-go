package tests_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/tests"
	"math"
)

func countUnits(p gomel.Poset) int {
	seenUnits := make(map[gomel.Hash]bool)
	queue := []gomel.Unit{}
	p.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
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
			if _, exists := seenUnits[*wParent.Hash()]; !exists {
				queue = append(queue, wParent)
				seenUnits[*wParent.Hash()] = true
			}
		}
	}
	return len(seenUnits)
}

func getMinMaxParents(p gomel.Poset) (int, int) {
	minParents, maxParents := math.MaxInt32, 0

	seenUnits := make(map[gomel.Hash]bool)
	queue := []gomel.Unit{}
	p.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			queue = append(queue, u)
			seenUnits[*u.Hash()] = true
		}
		return true
	})
	for len(queue) > 0 {
		w := queue[0]
		nParents := len(w.Parents())
		if nParents > 0 {
			if minParents > nParents {
				minParents = nParents
			}
			if maxParents < nParents {
				maxParents = nParents
			}
		}
		queue = queue[1:]
		seenUnits[*w.Hash()] = true
		for _, wParent := range w.Parents() {
			if _, exists := seenUnits[*wParent.Hash()]; !exists {
				queue = append(queue, wParent)
				seenUnits[*wParent.Hash()] = true
			}
		}
	}
	return minParents, maxParents
}

var _ = Describe("Generator", func() {
	Describe("CreateRandomNonForking", func() {
		var p gomel.Poset
		Context("Called with nProcesses = 10, minParents = 2, maxParents = 5, nUnits = 50", func() {
			p = CreateRandomNonForking(10, 2, 5, 50)
			It("Should retun poset with 10 processes", func() {
				Expect(p.GetNProcesses()).To(Equal(10))
			})
			It("Should have 50 units", func() {
				Expect(countUnits(p)).To(Equal(50))
			})
			It("Should have number of parents between 2 and 5", func() {
				minParents, maxParents := getMinMaxParents(p)
				Expect(minParents >= 2).To(BeTrue())
				Expect(maxParents <= 5).To(BeTrue())
			})
		})
	})
})
