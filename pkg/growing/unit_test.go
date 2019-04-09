package growing_test

import (
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/growing"
)

var _ = Describe("Units", func() {

	var (
		nProcesses int
		poset      gomel.Poset
		addFirst   [][]*preunit
		units      map[int]map[int][]gomel.Unit
		wg         sync.WaitGroup
	)

	AwaitAddUnit := func(pu gomel.Preunit, wg *sync.WaitGroup) {
		wg.Add(1)
		poset.AddUnit(pu, func(_ gomel.Preunit, result gomel.Unit, err error) {
			defer GinkgoRecover()
			defer wg.Done()
			Expect(err).NotTo(HaveOccurred())
			units[result.Creator()][result.Height()] = append(units[result.Creator()][result.Height()], result)
		})
	}

	BeforeEach(func() {
		nProcesses = 0
		poset = nil
		addFirst = nil
		units = make(map[int]map[int][]gomel.Unit)
		wg = sync.WaitGroup{}
	})

	JustBeforeEach(func() {
		for pid := 0; pid < nProcesses; pid++ {
			units[pid] = make(map[int][]gomel.Unit)
		}
		wg = sync.WaitGroup{}
		for _, pus := range addFirst {
			for _, pu := range pus {
				AwaitAddUnit(pu, &wg)
			}
			wg.Wait()
		}
	})

	Describe("small", func() {

		BeforeEach(func() {
			nProcesses = 4
			poset = NewPoset(nProcesses)
		})

		AfterEach(func() {
			poset.(*Poset).Stop()
		})

		Describe("Checking reflexivity of Below", func() {

			BeforeEach(func() {
				pu := &preunit{}
				pu.hash[0] = 1
				addFirst = [][]*preunit{[]*preunit{pu}}
			})

			It("Should return true", func() {
				Expect(len(units)).To(Equal(nProcesses))
				Expect(len(units[0])).To(Equal(1))
				Expect(len(units[0][0])).To(Equal(1))
				u := units[0][0][0]
				Expect(u.Below(u)).To(BeTrue())
			})

		})
		Describe("Checking lack of symmetry of Below", func() {

			BeforeEach(func() {
				pu0 := &preunit{}
				pu0.hash[0] = 1
				pu1 := &preunit{}
				pu1.hash[1] = 2
				pu1.creator = 1
				pu01 := &preunit{}
				pu01.hash[0] = 12
				pu01.parents = []gomel.Hash{pu0.hash, pu1.hash}
				addFirst = [][]*preunit{[]*preunit{pu0, pu1}, []*preunit{pu01}}
			})

			It("Should be true in one direction and false in the other", func() {
				u0 := units[0][0][0]
				u1 := units[1][0][0]
				u01 := units[0][1][0]
				Expect(u0.Below(u01)).To(BeTrue())
				Expect(u1.Below(u01)).To(BeTrue())
				Expect(u01.Below(u0)).To(BeFalse())
				Expect(u01.Below(u1)).To(BeFalse())
			})

		})
		Describe("Checking transitivity of Below", func() {

			BeforeEach(func() {
				pu0 := &preunit{}
				pu0.hash[0] = 1
				pu1 := &preunit{}
				pu1.hash[1] = 2
				pu1.creator = 1
				pu2 := &preunit{}
				pu2.hash[2] = 3
				pu2.creator = 2
				pu01 := &preunit{}
				pu01.hash[0] = 12
				pu01.parents = []gomel.Hash{pu0.hash, pu1.hash}
				pu02 := &preunit{}
				pu02.hash[0] = 13
				pu02.parents = []gomel.Hash{pu01.hash, pu2.hash}
				pu21 := &preunit{}
				pu21.hash[2] = 32
				pu21.creator = 2
				pu21.parents = []gomel.Hash{pu2.hash, pu01.hash}
				addFirst = [][]*preunit{[]*preunit{pu0, pu1, pu2}, []*preunit{pu01},
					[]*preunit{pu02, pu21}}
			})

			It("Should be true if two relations are true", func() {
				u0 := units[0][0][0]
				u01 := units[0][1][0]
				u02 := units[0][2][0]
				u21 := units[2][1][0]

				Expect(u0.Below(u01)).To(BeTrue())
				Expect(u01.Below(u02)).To(BeTrue())
				Expect(u0.Below(u02)).To(BeTrue())
				Expect(u01.Below(u21)).To(BeTrue())
				Expect(u0.Below(u21)).To(BeTrue())
			})

		})

		Describe("Checking Below works properly for forked dealing units.", func() {

			BeforeEach(func() {
				pu0 := &preunit{}
				pu0.hash[0] = 1
				pu0.creator = 0
				pu1 := &preunit{}
				pu1.hash[0] = 2
				pu1.creator = 0

				addFirst = [][]*preunit{[]*preunit{pu0}, []*preunit{pu1}}

			})

			It("Should return false for both below queries.", func() {
				u0 := units[0][0][0]
				u1 := units[0][0][1]

				Expect(u0.Below(u1)).To(BeFalse())
				Expect(u1.Below(u0)).To(BeFalse())
			})

		})

		Describe("Checking Below works properly for two forks going out of one unit.", func() {

			BeforeEach(func() {
				puBase0 := &preunit{}
				puBase0.hash[0] = 0
				puBase0.creator = 0
				puBase1 := &preunit{}
				puBase1.hash[0] = 1
				puBase1.creator = 1
				pu1 := &preunit{}
				pu1.hash[0] = 10
				pu1.creator = 0
				pu1.parents = []gomel.Hash{puBase0.hash, puBase1.hash}
				pu2 := &preunit{}
				pu2.hash[0] = 20
				pu2.creator = 0
				pu2.parents = []gomel.Hash{puBase0.hash, puBase1.hash}

				addFirst = [][]*preunit{[]*preunit{puBase0, puBase1}, []*preunit{pu1}, []*preunit{pu2}}

			})

			It("Should correctly answer all pairs of below queries.", func() {
				uBase := units[0][0][0]
				u1 := units[0][1][0]
				u2 := units[0][1][1]

				Expect(uBase.Below(u1)).To(BeTrue())
				Expect(uBase.Below(u2)).To(BeTrue())
				Expect(u1.Below(uBase)).To(BeFalse())
				Expect(u2.Below(uBase)).To(BeFalse())
				Expect(u1.Below(u2)).To(BeFalse())
				Expect(u2.Below(u1)).To(BeFalse())
			})

		})
	})

})
