package gob_test

import (
	"bytes"
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	. "gitlab.com/alephledger/consensus-go/pkg/encoding"
	. "gitlab.com/alephledger/consensus-go/pkg/encoding/gob"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Encoding/Decoding", func() {
	var (
		p          gomel.Poset
		readingErr error
		layers     [][]gomel.Unit
		encoder    Encoder
		decoder    Decoder
		network    *bytes.Buffer
		privKey    gomel.PrivateKey
	)
	BeforeEach(func() {
		p, readingErr = tests.CreatePosetFromTestFile("../../testdata/empty100.txt", tests.NewTestPosetFactory())
		Expect(readingErr).NotTo(HaveOccurred())
		layers = make([][]gomel.Unit, 0)
		network = &bytes.Buffer{}
		encoder = NewEncoder(network)
		decoder = NewDecoder(network)
		_, privKey, _ = signing.GenerateKeys()
	})
	Context("An empty silce of layers", func() {
		It("should be encoded/decoded to an empty slice of preunits", func() {
			eerr := encoder.EncodeUnits(layers)
			Expect(eerr).NotTo(HaveOccurred())
			preunits, derr := decoder.DecodePreunits()
			Expect(derr).NotTo(HaveOccurred())
			Expect(len(preunits)).To(Equal(len(layers)))
		})
	})
	Context("One layer with one unit", func() {
		BeforeEach(func() {
			pu := tests.NewPreunit(0, []gomel.Hash{}, []byte{})
			pu.SetSignature(privKey.Sign(pu))
			p.AddUnit(pu, func(pu gomel.Preunit, u gomel.Unit, _ error) {
				layers = append(layers, []gomel.Unit{u})
			})
		})
		It("should be encoded/decoded to one layer with one preunit corresponding to a given unit", func() {
			eerr := encoder.EncodeUnits(layers)
			Expect(eerr).NotTo(HaveOccurred())
			preunits, derr := decoder.DecodePreunits()
			Expect(derr).NotTo(HaveOccurred())
			Expect(len(preunits)).To(Equal(len(layers)))
			Expect(len(preunits[0])).To(Equal(len(layers[0])))
			pu := preunits[0][0]
			u := layers[0][0]
			Expect(eq(pu, u)).To(BeTrue())
		})
	})
	Context("One layer with 10 units", func() {
		BeforeEach(func() {
			layer := make([]gomel.Unit, 10)
			for i := 0; i < 10; i++ {
				pu := tests.NewPreunit(i, []gomel.Hash{}, []byte{})
				pu.SetSignature(privKey.Sign(pu))
				p.AddUnit(pu, func(pu gomel.Preunit, u gomel.Unit, _ error) {
					layer[i] = u
				})
			}
			layers = append(layers, layer)

		})
		It("should be encoded/decoded to one layer with 10 preunits corresponding to given units", func() {
			eerr := encoder.EncodeUnits(layers)
			Expect(eerr).NotTo(HaveOccurred())
			preunits, derr := decoder.DecodePreunits()
			Expect(derr).NotTo(HaveOccurred())
			Expect(len(preunits)).To(Equal(len(layers)))
			Expect(len(preunits[0])).To(Equal(len(layers[0])))
			for i, pu := range preunits[0] {
				u := layers[0][i]
				Expect(eq(pu, u)).To(BeTrue())
			}
		})
	})
	Context("10 layers with random number of units", func() {
		BeforeEach(func() {
			for i := 0; i < 10; i++ {
				nUnits := rand.Intn(10)
				layer := make([]gomel.Unit, nUnits)
				for j := 0; j < nUnits; j++ {
					pu := tests.NewPreunit(10*i+j, []gomel.Hash{}, []byte{})
					pu.SetSignature(privKey.Sign(pu))
					p.AddUnit(pu, func(pu gomel.Preunit, u gomel.Unit, _ error) {
						layer[j] = u
					})
				}
				layers = append(layers, layer)
			}

		})
		It("should be encoded/decoded to 10 layers with preunits corresponding to given units", func() {
			eerr := encoder.EncodeUnits(layers)
			Expect(eerr).NotTo(HaveOccurred())
			preunits, derr := decoder.DecodePreunits()
			Expect(derr).NotTo(HaveOccurred())
			Expect(len(preunits)).To(Equal(len(layers)))
			for i, layer := range preunits {
				Expect(len(layer)).To(Equal(len(layers[i])))
				for j, pu := range layer {
					u := layers[i][j]
					Expect(eq(pu, u)).To(BeTrue())
				}
			}
		})
	})

})

func eq(pu gomel.Preunit, u gomel.Unit) bool {
	if pu.Creator() != u.Creator() || !gomel.SigEq(pu.Signature(), u.Signature()) || len(pu.Parents()) != len(u.Parents()) {
		return false
	}
	for i, parent := range u.Parents() {
		if *parent.Hash() != pu.Parents()[i] {
			return false
		}
	}
	return true
}
