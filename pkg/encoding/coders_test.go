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
		dag, readingErr = tests.CreateDagFromTestFile("../testdata/regular1.txt", tests.NewTestDagFactory())
		Expect(readingErr).NotTo(HaveOccurred())
		network = &bytes.Buffer{}
	})
	Context("A dealing unit", func() {
		It("should be encoded/decoded to a preunit representing the original unit", func() {
			u := dag.PrimeUnits(0).Get(0)[0]
			err := SendUnit(u, network)
			Expect(err).NotTo(HaveOccurred())
			pu, err := ReceivePreunit(network)
			Expect(err).NotTo(HaveOccurred())
			Expect(pu.Creator()).To(Equal(u.Creator()))
			Expect(gomel.SigEq(pu.Signature(), u.Signature())).To(BeTrue())
			Expect(pu.Hash()).To(Equal(u.Hash()))
			Expect(len(pu.Parents())).To(Equal(len(u.Parents())))
			for i, parent := range u.Parents() {
				Expect(*parent.Hash()).To(Equal(pu.Parents()[i]))
			}
		})
	})
	Context("A non-dealing unit", func() {
		It("should be encoded/decoded to a preunit representing the original unit", func() {
			u := dag.MaximalUnitsPerProcess().Get(0)[0]
			err := SendUnit(u, network)
			Expect(err).NotTo(HaveOccurred())
			pu, err := ReceivePreunit(network)
			Expect(err).NotTo(HaveOccurred())
			Expect(pu.Creator()).To(Equal(u.Creator()))
			Expect(gomel.SigEq(pu.Signature(), u.Signature())).To(BeTrue())
			Expect(pu.Hash()).To(Equal(u.Hash()))
			Expect(len(pu.Parents())).To(Equal(len(u.Parents())))
			for i, parent := range u.Parents() {
				Expect(*parent.Hash()).To(Equal(*pu.Parents()[i]))
			}
		})
	})

})
