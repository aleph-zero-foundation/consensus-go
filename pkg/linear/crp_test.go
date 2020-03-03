package linear

import (
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Common Random Permutation", func() {

	checkWithProvidedTimingUnit := func(dag gomel.Dag, crpIt *commonRandomPermutation, rs *testRandomSource, shouldBeEqual bool) {
		permutation := []gomel.Unit{}
		crpIt.CRPIterate(2, nil, func(u gomel.Unit) bool {
			permutation = append(permutation, u)
			return true
		})

		tu := dag.UnitsOnLevel(1).Get(1)[0]
		permutation2 := []gomel.Unit{}
		crpIt.CRPIterate(2, tu, func(u gomel.Unit) bool {
			permutation2 = append(permutation2, u)
			return true
		})

		if shouldBeEqual {
			Expect(permutation).To(Equal(permutation2))
		} else {
			Expect(permutation).NotTo(Equal(permutation2))
		}

		tu = dag.UnitsOnLevel(1).Get(2)[0]
		permutation3 := []gomel.Unit{}
		crpIt.CRPIterate(2, tu, func(u gomel.Unit) bool {
			permutation3 = append(permutation3, u)
			return true
		})
		if shouldBeEqual {
			Expect(permutation2).To(Equal(permutation3))
			Expect(permutation).To(Equal(permutation3))
		} else {
			Expect(permutation2).NotTo(Equal(permutation3))
			Expect(permutation).NotTo(Equal(permutation3))
		}
	}

	Context("with deterministic part covering all units on level", func() {
		Context("providing a timing unit of creator different than pid 0", func() {
			It("should return different permutations", func() {
				dag, _, err := tests.CreateDagFromTestFile("../testdata/dags/4/regular.txt", tests.NewTestDagFactory())
				Expect(err).NotTo(HaveOccurred())
				rs := newTestRandomSource()
				crpFixedPrefix := uint16(4)
				crpIt := newCommonRandomPermutation(dag, rs, crpFixedPrefix)
				Expect(crpIt).NotTo(BeNil())

				checkWithProvidedTimingUnit(dag, crpIt, rs, true)
				Expect(rs.called).To(BeFalse())
			})
		})

		Context("with empty dag", func() {
			It("should return true and provide no units", func() {
				nProc := uint16(4)
				dag, _ := tests.NewTestDagFactory().CreateDag(nProc)
				rs := tests.NewTestRandomSource()
				crpFixedPrefix := uint16(10)
				crpIt := newCommonRandomPermutation(dag, rs, crpFixedPrefix)

				Expect(crpIt).NotTo(BeNil())

				called := false
				result := crpIt.CRPIterate(0, nil, func(gomel.Unit) bool {
					called = true
					return true
				})
				Expect(called).To(BeFalse())
				Expect(result).To(BeTrue())
			})
		})

		Context("with enough units on level", func() {
			It("should return true and all units", func() {
				nProc := 4
				dag, _, err := tests.CreateDagFromTestFile("../testdata/dags/4/regular.txt", tests.NewTestDagFactory())
				Expect(err).NotTo(HaveOccurred())
				rs := tests.NewTestRandomSource()

				crpFixedPrefix := uint16(10)
				crpIt := newCommonRandomPermutation(dag, rs, crpFixedPrefix)

				Expect(crpIt).NotTo(BeNil())

				perm := map[gomel.Hash]bool{}
				called := 0
				result := crpIt.CRPIterate(0, nil, func(u gomel.Unit) bool {
					perm[*u.Hash()] = true
					called++
					return true
				})
				Expect(result).To(BeTrue())
				Expect(len(perm)).To(Equal(nProc))
				Expect(called).To(Equal(nProc))
			})
		})
	})

	Context("with deterministic part not covering all units on level", func() {
		It("should use the RandomSource to determine the permutation", func() {
			dag, _, err := tests.CreateDagFromTestFile("../testdata/dags/4/regular.txt", tests.NewTestDagFactory())
			Expect(err).NotTo(HaveOccurred())
			rs := newTestRandomSource()

			crpFixedPrefix := uint16(1)
			crpIt := newCommonRandomPermutation(dag, rs, crpFixedPrefix)

			Expect(crpIt).NotTo(BeNil())

			called := false
			crpIt.CRPIterate(0, nil, func(u gomel.Unit) bool {
				called = true
				return true
			})
			Expect(called).To(BeTrue())
			Expect(rs.called).To(BeTrue())
		})

		Context("using different RandomSource instances", func() {
			It("should return different permutations", func() {
				dag, _, err := tests.CreateDagFromTestFile("../testdata/dags/4/regular.txt", tests.NewTestDagFactory())
				Expect(err).NotTo(HaveOccurred())

				rsData := map[int][]byte{}
				for level := 0; level < 10; level++ {
					randData := make([]byte, 64)
					rand.Read(randData)
					rsData[level] = randData
				}
				rs := newDeterministicRandomSource(rsData)

				crpFixedPrefix := uint16(0)
				crpIt := newCommonRandomPermutation(dag, rs, crpFixedPrefix)

				Expect(crpIt).NotTo(BeNil())

				permutation := []gomel.Unit{}
				perm := map[gomel.Hash]bool{}
				crpIt.CRPIterate(0, nil, func(u gomel.Unit) bool {
					permutation = append(permutation, u)
					perm[*u.Hash()] = true
					return true
				})
				Expect(rs.called).To(BeTrue())

				for level := range rsData {
					data := append([]byte{}, rsData[level]...)
					for ix := range data {
						data[ix] = data[ix] ^ 0xFF
					}
					rsData[level] = data
				}
				rs = newDeterministicRandomSource(rsData)
				crpIt = newCommonRandomPermutation(dag, rs, crpFixedPrefix)

				permutation2 := []gomel.Unit{}
				perm2 := map[gomel.Hash]bool{}
				crpIt.CRPIterate(0, nil, func(u gomel.Unit) bool {
					permutation2 = append(permutation2, u)
					perm2[*u.Hash()] = true
					return true
				})

				Expect(perm).To(Equal(perm2))
				Expect(permutation).ToNot(Equal(permutation2))
			})
		})

		Context("providing a timing unit of creator different than pid 0", func() {
			It("should return different permutations", func() {
				dag, _, err := tests.CreateDagFromTestFile("../testdata/dags/4/regular.txt", tests.NewTestDagFactory())
				Expect(err).NotTo(HaveOccurred())
				rs := newTestRandomSource()
				crpFixedPrefix := uint16(0)
				crpIt := newCommonRandomPermutation(dag, rs, crpFixedPrefix)
				Expect(crpIt).NotTo(BeNil())

				checkWithProvidedTimingUnit(dag, crpIt, rs, true)
				Expect(rs.called).To(BeTrue())
			})
		})

		Context("missing random bytes", func() {
			It("should return false, but provide deterministic part of the permutation", func() {
				dag, _, err := tests.CreateDagFromTestFile("../testdata/dags/10/only_dealing.txt", tests.NewTestDagFactory())
				Expect(err).NotTo(HaveOccurred())
				Expect(dag).NotTo(BeNil())

				rs := newDeterministicRandomSource(map[int][]byte{})
				crpFixedPrefix := uint16(4)
				crpIt := newCommonRandomPermutation(dag, rs, crpFixedPrefix)
				Expect(crpIt).NotTo(BeNil())

				permutation := []gomel.Unit{}
				ok := crpIt.CRPIterate(0, nil, func(u gomel.Unit) bool {
					permutation = append(permutation, u)
					return true
				})
				Expect(ok).To(BeFalse())
				Expect(len(permutation)).To(Equal(4))
			})
		})

		Context("two dags with slightly different view, where one is subset of the other", func() {
			It("deterministic part of the permutation defines set of pids, not set of units", func() {
				dag, _, err := tests.CreateDagFromTestFile("../testdata/dags/10/only_dealing.txt", tests.NewTestDagFactory())
				Expect(err).NotTo(HaveOccurred())
				Expect(dag).NotTo(BeNil())

				dag2, _, err2 := tests.CreateDagFromTestFile(
					"../testdata/dags/10/only_dealing_but_not_all.txt",
					tests.NewTestDagFactory(),
				)
				Expect(err2).NotTo(HaveOccurred())
				Expect(dag2).NotTo(BeNil())

				rs := newTestRandomSource()
				crpFixedPrefix := uint16(4)
				crpIt := newCommonRandomPermutation(dag, rs, crpFixedPrefix)
				Expect(crpIt).NotTo(BeNil())

				permutation := []gomel.Unit{}
				perm := map[gomel.Hash]bool{}
				ok := crpIt.CRPIterate(0, nil, func(u gomel.Unit) bool {
					permutation = append(permutation, u)
					perm[*u.Hash()] = true
					return true
				})
				Expect(ok).To(BeTrue())

				rs = newTestRandomSource()
				crpIt = newCommonRandomPermutation(dag2, rs, crpFixedPrefix)
				Expect(crpIt).NotTo(BeNil())

				permutation2 := []gomel.Unit{}
				perm2 := map[gomel.Hash]bool{}
				ok = crpIt.CRPIterate(0, nil, func(u gomel.Unit) bool {
					permutation2 = append(permutation2, u)
					perm2[*u.Hash()] = true
					return true
				})
				Expect(ok).To(BeTrue())

				// check if suffixes are same
				suffix := permutation[crpFixedPrefix:]
				suffix2 := permutation2[crpFixedPrefix-1:]

				Expect(suffix).To(Equal(suffix2))
			})
		})
	})
})

type testRandomSource struct {
	gomel.RandomSource
	called bool
}

func newTestRandomSource() *testRandomSource {
	return &testRandomSource{RandomSource: tests.NewTestRandomSource(), called: false}
}

func (rs *testRandomSource) RandomBytes(pid uint16, level int) []byte {
	rs.called = true
	return rs.RandomSource.RandomBytes(pid, level)
}

type deterministicRandomSource struct {
	*testRandomSource
	randomBytes map[int][]byte
}

func newDeterministicRandomSource(randomBytes map[int][]byte) *deterministicRandomSource {
	return &deterministicRandomSource{
		testRandomSource: newTestRandomSource(),
		randomBytes:      randomBytes,
	}
}

func (rs *deterministicRandomSource) RandomBytes(pid uint16, level int) []byte {
	rs.testRandomSource.RandomBytes(pid, level)
	return rs.randomBytes[level]
}

func (rs *deterministicRandomSource) DataToInclude(parents []gomel.Unit, level int) ([]byte, error) {
	return rs.testRandomSource.DataToInclude(parents, level)
}
