package custom_test

import (
	"bytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/encoding"
	. "gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Encoding/Decoding", func() {
	var (
		p          gomel.Poset
		readingErr error
		encoder    Encoder
		decoder    Decoder
		network    *bytes.Buffer
	)
	BeforeEach(func() {
		p, readingErr = tests.CreatePosetFromTestFile("../../testdata/regular1.txt", tests.NewTestPosetFactory())
		Expect(readingErr).NotTo(HaveOccurred())
		network = &bytes.Buffer{}
		encoder = NewEncoder(network)
		decoder = NewDecoder(network)
	})
	Context("A dealing unit", func() {
		It("should be encoded/decoded to a preunit representing the original unit", func() {
			u := p.PrimeUnits(0).Get(0)[0]
			err := encoder.EncodeUnit(u)
			Expect(err).NotTo(HaveOccurred())
			pu, err := decoder.DecodePreunit()
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
			u := p.MaximalUnitsPerProcess().Get(0)[0]
			err := encoder.EncodeUnit(u)
			Expect(err).NotTo(HaveOccurred())
			pu, err := decoder.DecodePreunit()
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
