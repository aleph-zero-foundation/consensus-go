package urn_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/random/urn"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Tcoin", func() {
	var (
		pid int
		dag gomel.Dag
		rs  gomel.RandomSource
		err error
	)
	BeforeEach(func() {
		pid = 0
		dag, err = tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
		Expect(err).NotTo(HaveOccurred())
		rs = urn.NewUrn(pid)
		rs.Init(dag)
	})
	Describe("GetCRP", func() {
		Context("On a given level", func() {
			It("Should return a permutation of pids", func() {
				perm := rs.GetCRP(0)
				Expect(len(perm)).To(Equal(dag.NProc()))
				elems := make(map[int]bool)
				for _, pid := range perm {
					elems[pid] = true
				}
				Expect(len(elems)).To(Equal(dag.NProc()))
			})
		})
	})
})
