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
		dag, _, readingErr = tests.CreateDagFromTestFile("../testdata/dags/4/regular.txt", tests.NewTestDagFactory())
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
	Context("Sending/Receving chunks", func() {
		Context("when an antichain of dealing units is being sent", func() {
			It("should receive a slice containing a single slice of dealing preunits", func() {
				toSend := []gomel.Unit{}
				dag.PrimeUnits(0).Iterate(func(units []gomel.Unit) bool {
					toSend = append(toSend, units[0])
					return true
				})
				// sending to a buffer
				var buf bytes.Buffer
				err := SendChunk(toSend, &buf)
				Expect(err).NotTo(HaveOccurred())
				// receiving
				pus, err := ReceiveChunk(&buf)
				//checks
				Expect(len(pus)).To(Equal(len(toSend)))
				for _, pu := range pus {
					Expect(pu.Hash()).To(Equal(dag.PrimeUnits(0).Get(pu.Creator())[0].Hash()))
				}
			})
		})
		Context("when units created by a single process are being sent", func() {
			It("should receive a slice containing slices of single preunits", func() {
				// Collecting units created by 0.
				mu := dag.MaximalUnitsPerProcess().Get(0)[0]
				toSend := []gomel.Unit{}
				for mu != nil {
					toSend = append(toSend, mu)
					mu = gomel.Predecessor(mu)
				}
				// sending to a buffer
				var buf bytes.Buffer
				err := SendChunk(toSend, &buf)
				Expect(err).NotTo(HaveOccurred())
				// receiving
				pus, err := ReceiveChunk(&buf)
				// checks
				Expect(len(pus)).To(Equal(len(toSend)))
				for h, pu := range pus {
					Expect(pu.Hash()).To(Equal(toSend[len(pus)-1-h].Hash()))
				}
			})
		})
	})
	Context("Decoding", func() {
		Context("on a unit with too much data", func() {
			It("should return an error", func() {
				nProc := 0
				// creator, epochID, signature, nParents, parentsHeights, controlHash, data length
				encoded := make([]byte, 2+8+64+(2+4*nProc+32)+4)
				dataLenStartOffset := 2 + 8 + 64 + (2 + 4*nProc + 32)
				binary.LittleEndian.PutUint32(encoded[dataLenStartOffset:], config.MaxDataBytesPerUnit+1)
				_, err := DecodePreunit(encoded)
				Expect(err).To(MatchError("maximal allowed data size in a preunit exceeded"))
			})
		})
		Context("on a unit with too long random source data", func() {
			It("should return an error", func() {
				nProc := 0
				// creator, epochID, signature, nParents, parentsHeights, controlHash, data length, random source data length
				encoded := make([]byte, 2+8+64+2+4*nProc+32+4+4)
				rsDataLenStartOffset := 2 + 8 + 64 + 2 + 4*nProc + 32 + 4
				binary.LittleEndian.PutUint32(encoded[rsDataLenStartOffset:], config.MaxRandomSourceDataBytesPerUnit+1)
				_, err := DecodePreunit(encoded)
				Expect(err).To(MatchError("maximal allowed random source data size in a preunit exceeded"))
			})
		})
	})
	Context("ReceiveChunk", func() {
		Context("On a chunk with too many units", func() {
			It("should return an error", func() {
				encoded := make([]byte, 4)
				binary.LittleEndian.PutUint32(encoded[:], config.MaxUnitsInChunk+1)
				_, err := ReceiveChunk(bytes.NewBuffer(encoded))
				Expect(err).To(MatchError("chunk contains too many units"))
			})
		})

	})
})
