package encoding_test

import (
	"bytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Encoding/Decoding", func() {
	var (
		dag        gomel.Dag
		readingErr error
		network    *bytes.Buffer
	)
	BeforeEach(func() {
		dag, readingErr = tests.CreateDagFromTestFile("../testdata/dags/4/regular.txt", tests.NewTestDagFactory())
		Expect(readingErr).NotTo(HaveOccurred())
		network = &bytes.Buffer{}
	})
	Context("A dealing unit", func() {
		It("should be encoded/decoded to a preunit representing the original unit", func() {
			u := dag.PrimeUnits(0).Get(0)[0]
			err := SendUnit(u, network)
			Expect(err).NotTo(HaveOccurred())
			pu, err := ReceivePreunit(network, dag.NProc())
			Expect(err).NotTo(HaveOccurred())
			Expect(pu.Creator()).To(Equal(u.Creator()))
			Expect(gomel.SigEq(pu.Signature(), u.Signature())).To(BeTrue())
			Expect(pu.Hash()).To(Equal(u.Hash()))
			Expect(pu.ControlHash()).To(Equal(u.ControlHash()))
			for i, h := range pu.ParentsHeights() {
				if h == -1 {
					Expect(u.Parents()[i]).To(BeNil())
				} else {
					Expect(u.Parents()[i].Height()).To(Equal(h))
				}
			}
		})
	})
	Context("A non-dealing unit", func() {
		It("should be encoded/decoded to a preunit representing the original unit", func() {
			u := dag.MaximalUnitsPerProcess().Get(0)[0]
			err := SendUnit(u, network)
			Expect(err).NotTo(HaveOccurred())
			pu, err := ReceivePreunit(network, dag.NProc())
			Expect(err).NotTo(HaveOccurred())
			Expect(pu.Creator()).To(Equal(u.Creator()))
			Expect(gomel.SigEq(pu.Signature(), u.Signature())).To(BeTrue())
			Expect(pu.Hash()).To(Equal(u.Hash()))
			Expect(pu.ControlHash()).To(Equal(u.ControlHash()))
			for i, h := range pu.ParentsHeights() {
				if h == -1 {
					Expect(u.Parents()[i]).To(BeNil())
				} else {
					Expect(u.Parents()[i].Height()).To(Equal(h))
				}
			}
		})
	})

})
