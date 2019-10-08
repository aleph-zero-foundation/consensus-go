package encoding_test

import (
	"bytes"
	"encoding/binary"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/config"
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
			pu, err := ReceivePreunit(network)
			Expect(err).NotTo(HaveOccurred())
			Expect(pu.Creator()).To(Equal(u.Creator()))
			Expect(gomel.SigEq(pu.Signature(), u.Signature())).To(BeTrue())
			Expect(pu.Hash()).To(Equal(u.Hash()))
			Expect(pu.View().ControlHash).To(Equal(u.View().ControlHash))
			for i, h := range pu.View().Heights {
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
			pu, err := ReceivePreunit(network)
			Expect(err).NotTo(HaveOccurred())
			Expect(pu.Creator()).To(Equal(u.Creator()))
			Expect(gomel.SigEq(pu.Signature(), u.Signature())).To(BeTrue())
			Expect(pu.Hash()).To(Equal(u.Hash()))
			Expect(pu.View().ControlHash).To(Equal(u.View().ControlHash))
			for i, h := range pu.View().Heights {
				if h == -1 {
					Expect(u.Parents()[i]).To(BeNil())
				} else {
					Expect(u.Parents()[i].Height()).To(Equal(h))
				}
			}
		})
	})
	Context("Decoding", func() {
		Context("on a unit with too much data", func() {
			It("should return an error", func() {
				nProc := uint16(4)
				// creator, signature, parentsHeights, controlHash, data length
				encoded := make([]byte, 2+64+4*nProc+32+4)
				dataLenStartOffset := 2 + 64 + 4*nProc + 32
				binary.LittleEndian.PutUint32(encoded[dataLenStartOffset:], config.MaxDataBytesPerUnit+1)
				_, err := DecodePreunit(encoded, nProc)
				Expect(err).To(MatchError("maximal allowed data size in a preunit exceeded"))
			})
		})
		Context("on a unit with to long random source data", func() {
			It("should return an error", func() {
				nProc := uint16(4)
				// creator, signature, parentsHeights, controlHash, data length, random source data length
				encoded := make([]byte, 2+64+4*nProc+32+4+4)
				rsDataLenStartOffset := 2 + 64 + 4*nProc + 32 + 4
				binary.LittleEndian.PutUint32(encoded[rsDataLenStartOffset:], config.MaxRandomSourceDataBytesPerUnit+1)
				_, err := DecodePreunit(encoded, nProc)
				Expect(err).To(MatchError("maximal allowed random source data size in a preunit exceeded"))
			})
		})
	})
	Context("ReceiveChunk", func() {
		Context("On a chunk with too many antichains", func() {
			It("should return an error", func() {
				nProc := uint16(4)
				encoded := make([]byte, 4)
				binary.LittleEndian.PutUint32(encoded[:], config.MaxAntichainsInChunk+1)
				_, _, err := ReceiveChunk(bytes.NewBuffer(encoded), nProc)
				Expect(err).To(MatchError("chunk contains too many antichains"))
			})
		})
		Context("On a chunk with one antichain containing too many units", func() {
			It("should return an error", func() {
				nProc := uint16(4)
				// nAntichains, antichain size
				encoded := make([]byte, 4+4)
				binary.LittleEndian.PutUint32(encoded[0:4], 1)
				binary.LittleEndian.PutUint32(encoded[4:8], uint32(nProc)*uint32(nProc)+1)
				_, _, err := ReceiveChunk(bytes.NewBuffer(encoded), nProc)
				Expect(err).To(MatchError("antichain length too long"))
			})
		})
	})
})
