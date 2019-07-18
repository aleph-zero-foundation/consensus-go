package beacon_test

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	. "gitlab.com/alephledger/consensus-go/pkg/random/beacon"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Beacon", func() {
	var (
		n        int
		maxLevel int
		dag      []gomel.Dag
		rs       []gomel.RandomSource
		err      error
	)
	BeforeEach(func() {
		n = 4
		maxLevel = 13
		dag = make([]gomel.Dag, n)
		rs = make([]gomel.RandomSource, n)
		for pid := 0; pid < n; pid++ {
			dag[pid], err = tests.CreateDagFromTestFile("../../testdata/empty4.txt", tests.NewTestDagFactory())
			Expect(err).NotTo(HaveOccurred())
			rs[pid] = NewBeacon(dag[pid], pid)
		}
		// Generating very regular dag
		for level := 0; level < maxLevel; level++ {
			for creator := 0; creator < n; creator++ {
				pu, err := creating.NewNonSkippingUnit(dag[creator], creator, []byte{}, rs[creator])
				Expect(err).NotTo(HaveOccurred())
				for pid := 0; pid < n; pid++ {
					var wg sync.WaitGroup
					wg.Add(1)
					var added gomel.Unit
					dag[pid].AddUnit(pu, rs[pid], func(_ gomel.Preunit, u gomel.Unit, err error) {
						defer wg.Done()
						added = u
						Expect(err).NotTo(HaveOccurred())
					})
					errComp := rs[pid].CheckCompliance(added)
					Expect(errComp).NotTo(HaveOccurred())
					rs[pid].Update(added)
					wg.Wait()
				}
			}
		}
	})
	Describe("GetCRP", func() {
		Context("On a given level", func() {
			It("Should return a permutation of pids", func() {
				perm := rs[0].GetCRP(8)
				Expect(len(perm)).To(Equal(dag[0].NProc()))
				elems := make(map[int]bool)
				for _, pid := range perm {
					elems[pid] = true
				}
				Expect(len(elems)).To(Equal(dag[0].NProc()))
			})
			It("Should return the same permutation for all pid", func() {
				perm := make([][]int, n)
				for pid := 0; pid < n; pid++ {
					perm[pid] = rs[pid].GetCRP(9)
				}
				for pid := 1; pid < n; pid++ {
					for i := range perm[pid] {
						Expect(perm[pid][i]).To(Equal(perm[pid-1][i]))
					}
				}
			})
			Context("On too high level", func() {
				It("Should return nil", func() {
					perm := rs[0].GetCRP(11)
					Expect(perm).To(BeNil())
				})
			})
			Context("On too low level", func() {
				It("Should return nil", func() {
					perm := rs[0].GetCRP(3)
					Expect(perm).To(BeNil())
				})
			})
		})
	})
	Describe("CheckCompliance", func() {
		Context("On a dealing unit without tcoin included", func() {
			It("Should return an error", func() {
				u := dag[0].PrimeUnits(0).Get(0)[0]
				um := newUnitMock(u, []byte{})
				err := rs[0].CheckCompliance(um)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(HavePrefix("Decoding tcoin failed")))
			})
		})
		Context("On a voting unit", func() {
			Context("Having no votes", func() {
				It("Should return an error", func() {
					u := dag[0].PrimeUnits(3).Get(0)[0]
					um := newUnitMock(u, []byte{})
					err := rs[0].CheckCompliance(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(HavePrefix("votes wrongly encoded")))
				})
			})
			Context("Having one vote missing", func() {
				It("Should return an error", func() {
					u := dag[0].PrimeUnits(3).Get(0)[0]
					votes := u.RandomSourceData()
					votes[0] = 0
					um := newUnitMock(u, votes)
					err := rs[0].CheckCompliance(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("missing vote"))
				})
			})
			Context("Having incorrect vote", func() {
				It("Should return an error", func() {
					u := dag[0].PrimeUnits(3).Get(0)[0]
					votes := u.RandomSourceData()
					votes[dag[0].NProc()-1] = 2
					// preparing fake proof
					proof := big.NewInt(int64(20190718))
					proofBytes, _ := proof.GobEncode()
					var buf bytes.Buffer
					binary.Write(&buf, binary.LittleEndian, uint16(len(proofBytes)))
					buf.Write(proofBytes)
					votes = append(votes, buf.Bytes()...)

					um := newUnitMock(u, votes)
					err := rs[0].CheckCompliance(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("the provided proof is incorrect"))
				})
			})
		})
		Context("On a unit which should contain shares", func() {
			Context("Without random source data", func() {
				It("Should return an error", func() {
					u := dag[0].PrimeUnits(8).Get(0)[0]
					um := newUnitMock(u, []byte{})
					err := rs[0].CheckCompliance(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("cses wrongly encoded"))
				})
			})
			Context("With missing shares", func() {
				It("Should return an error", func() {
					u := dag[0].PrimeUnits(8).Get(0)[0]
					shares := make([]byte, dag[0].NProc())
					um := newUnitMock(u, shares)
					err := rs[0].CheckCompliance(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("missing share"))
				})
			})
			Context("With incorrect shares", func() {
				It("Should return an error", func() {
					u := dag[0].PrimeUnits(8).Get(0)[0]
					// taking shares of a unit of different level
					v := dag[0].PrimeUnits(9).Get(0)[0]
					um := newUnitMock(u, v.RandomSourceData())
					err := rs[0].CheckCompliance(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("invalid share"))
				})
			})
		})
	})
})

type unitMock struct {
	u      gomel.Unit
	rsData []byte
}

func newUnitMock(u gomel.Unit, rsData []byte) *unitMock {
	return &unitMock{
		u:      u,
		rsData: rsData,
	}
}

func (um *unitMock) Creator() int {
	return um.u.Creator()
}

func (um *unitMock) Signature() gomel.Signature {
	return um.u.Signature()
}

func (um *unitMock) Hash() *gomel.Hash {
	return um.u.Hash()
}

func (um *unitMock) Data() []byte {
	return um.u.Data()
}

func (um *unitMock) RandomSourceData() []byte {
	return um.rsData
}

func (um *unitMock) Height() int {
	return um.u.Height()
}

func (um *unitMock) Parents() []gomel.Unit {
	return um.u.Parents()
}

func (um *unitMock) Level() int {
	return um.u.Level()
}

func (um *unitMock) Below(v gomel.Unit) bool {
	return um.u.Below(v)
}

func (um *unitMock) Above(v gomel.Unit) bool {
	return um.u.Above(v)
}

func (um *unitMock) HasForkingEvidence(creator int) bool {
	return um.u.HasForkingEvidence(creator)
}

func (um *unitMock) Floor() [][]gomel.Unit {
	return um.u.Floor()
}
