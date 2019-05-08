package request_test

import (
	"io"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/request"
)

type poset struct {
	primeUnits   []gomel.SlottedUnits
	maximalUnits gomel.SlottedUnits
	attemptedAdd []gomel.Preunit
}

func (p *poset) AddUnit(unit gomel.Preunit, callback func(gomel.Preunit, gomel.Unit, error)) {
	p.attemptedAdd = append(p.attemptedAdd, unit)
	callback(unit, nil, nil)
}

func (p *poset) PrimeUnits(level int) gomel.SlottedUnits {
	return p.primeUnits[level]
}

func (p *poset) MaximalUnitsPerProcess() gomel.SlottedUnits {
	return p.maximalUnits
}

func (p *poset) Get(hashes []gomel.Hash) []gomel.Unit {
	result := make([]gomel.Unit, len(hashes))
	visited := map[gomel.Unit]bool{}
	toVisit := []gomel.Unit{}
	p.maximalUnits.Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			toVisit = append(toVisit, u)
			visited[u] = true
		}
		return true
	})
	for len(toVisit) > 0 {
		visiting := toVisit[0]
		toVisit = toVisit[1:]
		for _, u := range visiting.Parents() {
			if !visited[u] {
				toVisit = append(toVisit, u)
				visited[u] = true
			}
		}
		for i, h := range hashes {
			if *visiting.Hash() == h {
				result[i] = visiting
			}
		}
	}
	return result
}

func (p *poset) IsQuorum(_ int) bool {
	return false
}

type slottedUnits struct {
	contents [][]gomel.Unit
}

func (su *slottedUnits) Get(id int) []gomel.Unit {
	return su.contents[id]
}

func (su *slottedUnits) Set(id int, units []gomel.Unit) {
	su.contents[id] = units
}

func (su *slottedUnits) Iterate(work func([]gomel.Unit) bool) {
	for _, units := range su.contents {
		if !work(units) {
			return
		}
	}
}

func newSlottedUnits(n int) gomel.SlottedUnits {
	return &slottedUnits{
		contents: make([][]gomel.Unit, n),
	}
}

type unit struct {
	creator            int
	signature          gomel.Signature
	hash               gomel.Hash
	height             int
	parents            []gomel.Unit
	level              int
	hasForkingEvidence map[int]bool
}

func (u *unit) Below(v gomel.Unit) bool {
	toVisit := []gomel.Unit{v}
	visiting := map[gomel.Hash]bool{}
	visiting[*v.Hash()] = true
	for len(toVisit) > 0 {
		w := toVisit[0]
		toVisit = toVisit[1:]
		if w == u {
			return true
		}
		for _, p := range w.Parents() {
			if !visiting[*p.Hash()] {
				toVisit = append(toVisit, p)
				visiting[*p.Hash()] = true
			}
		}
	}
	return false
}

func (u *unit) Above(v gomel.Unit) bool {
	return v.Below(u)
}

func (u *unit) Creator() int {
	return u.creator
}

func (u *unit) Signature() gomel.Signature {
	return u.signature
}

func (u *unit) Hash() *gomel.Hash {
	return &u.hash
}

func (u *unit) Height() int {
	return u.height
}

func (u *unit) Parents() []gomel.Unit {
	return u.parents
}

func (u *unit) Level() int {
	return u.level
}

func (u *unit) HasForkingEvidence(creator int) bool {
	return u.hasForkingEvidence[creator]
}

type connection struct {
	in  io.Reader
	out io.Writer
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

func newConnection() (network.Connection, network.Connection) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	return &connection{r1, w2}, &connection{r2, w1}
}

var _ = Describe("Protocol", func() {

	var (
		pu1 []gomel.SlottedUnits
		pu2 []gomel.SlottedUnits
		mu1 gomel.SlottedUnits
		mu2 gomel.SlottedUnits
		p1  *poset
		p2  *poset
		in  gsync.Protocol
		out gsync.Protocol
		c1  network.Connection
		c2  network.Connection
	)

	BeforeEach(func() {
		mu1 = nil
		mu2 = nil
		pu1 = nil
		pu2 = nil
		in = In{}
		out = Out{}
		c1, c2 = newConnection()
	})

	JustBeforeEach(func() {
		p1 = &poset{
			primeUnits:   pu1,
			maximalUnits: mu1,
			attemptedAdd: nil,
		}
		p2 = &poset{
			primeUnits:   pu2,
			maximalUnits: mu2,
			attemptedAdd: nil,
		}
	})

	Describe("in a small poset", func() {

		var (
			nProcesses         int
			maxUnitsInPoset1   []gomel.Unit
			maxUnitsInPoset2   []gomel.Unit
			primeUnitsInPoset1 []gomel.Unit
			primeUnitsInPoset2 []gomel.Unit
		)

		BeforeEach(func() {
			nProcesses = 4
			pu1 = []gomel.SlottedUnits{}
			pu2 = []gomel.SlottedUnits{}
			for i := 0; i < 10; i++ {
				pu1 = append(pu1, newSlottedUnits(nProcesses))
				pu2 = append(pu2, newSlottedUnits(nProcesses))
			}
			mu1 = newSlottedUnits(nProcesses)
			mu2 = newSlottedUnits(nProcesses)
			maxUnitsInPoset1 = nil
			maxUnitsInPoset2 = nil
			primeUnitsInPoset1 = nil
			primeUnitsInPoset2 = nil
		})

		JustBeforeEach(func() {
			for _, u := range maxUnitsInPoset1 {
				id := u.Creator()
				mu1.Set(id, append(mu1.Get(id), u))
			}
			for _, u := range maxUnitsInPoset2 {
				id := u.Creator()
				mu2.Set(id, append(mu2.Get(id), u))
			}
			for _, u := range primeUnitsInPoset1 {
				id := u.Creator()
				level := u.Level()
				pu1[level].Set(id, append(pu1[level].Get(id), u))
			}
			for _, u := range primeUnitsInPoset2 {
				id := u.Creator()
				level := u.Level()
				pu2[level].Set(id, append(pu2[level].Get(id), u))
			}
		})

		Context("when both copies are empty", func() {

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
				singleUnit := &unit{
					creator: 0,
					height:  0,
					parents: nil,
					level:   0,
				}
				singleUnit.hash[0] = 1
				primeUnitsInPoset1 = append(primeUnitsInPoset1, singleUnit)
				maxUnitsInPoset1 = append(maxUnitsInPoset1, singleUnit)
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
				singleUnit := &unit{
					creator: 1,
					height:  0,
					parents: nil,
					level:   0,
				}
				primeUnitsInPoset2 = append(primeUnitsInPoset2, singleUnit)
				maxUnitsInPoset2 = append(maxUnitsInPoset2, singleUnit)
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
				for id := 0; id < nProcesses; id++ {
					someUnit := &unit{
						creator: id,
						height:  0,
						parents: nil,
						level:   0,
					}
					someUnit.hash[0] = byte(id + 1)
					primeUnitsInPoset1 = append(primeUnitsInPoset1, someUnit)
					primeUnitsInPoset2 = append(primeUnitsInPoset2, someUnit)
					maxUnitsInPoset1 = append(maxUnitsInPoset1, someUnit)
					maxUnitsInPoset2 = append(maxUnitsInPoset2, someUnit)
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
