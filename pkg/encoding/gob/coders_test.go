package gob_test

import (
	"bytes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	. "gitlab.com/alephledger/consensus-go/pkg/encoding"
	. "gitlab.com/alephledger/consensus-go/pkg/encoding/gob"
	"math/rand"
)

var _ = Describe("Encoding/Decoding", func() {
	var (
		layers  [][]gomel.Unit
		encoder Encoder
		decoder Decoder
		network *bytes.Buffer
		privKey signing.PrivateKey
	)
	BeforeEach(func() {
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
			pu := &preunit{}
			pu.SetSignature(privKey.Sign(pu))
			u := unit{}
			u.hash = pu.hash
			u.signature = pu.signature
			layers = append(layers, []gomel.Unit{&u})

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
				pu := &preunit{}
				pu.hash[0] = byte(i)
				pu.SetSignature(privKey.Sign(pu))
				u := unit{}
				u.hash = pu.hash
				u.signature = pu.signature
				layer[i] = &u
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
					pu := &preunit{}
					pu.hash[i] = byte(j)
					pu.SetSignature(privKey.Sign(pu))
					u := unit{}
					u.hash = pu.hash
					u.signature = pu.signature
					layer[j] = &u
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

type preunit struct {
	creator   int
	signature gomel.Signature
	hash      gomel.Hash
	parents   []gomel.Hash
}

func (pu *preunit) Creator() int {
	return pu.creator
}

func (pu *preunit) Signature() gomel.Signature {
	return pu.signature
}

func (pu *preunit) Hash() *gomel.Hash {
	return &pu.hash
}

func (pu *preunit) SetSignature(sig gomel.Signature) {
	pu.signature = sig
}

func (pu *preunit) Parents() []gomel.Hash {
	return pu.parents
}

type unit struct {
	creator   int
	level     int
	hash      gomel.Hash
	signature gomel.Signature
	parents   []gomel.Unit
}

func newUnit(creator int, id int) *unit {
	var h gomel.Hash
	h[0] = byte(id)
	return &unit{
		creator: creator,
		level:   0,
		hash:    h,
		parents: []gomel.Unit{},
	}
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
	if len(u.Parents()) == 0 {
		return 0
	}
	return 1 + u.Parents()[0].Height()
}

func (u *unit) Parents() []gomel.Unit {
	return u.parents
}

func (u *unit) Level() int {
	return u.level
}

func (u *unit) HasForkingEvidence(creator int) bool {
	return false
}

func (u *unit) Below(v gomel.Unit) bool {
	if *u.Hash() == *v.Hash() {
		return true
	}
	for _, w := range v.Parents() {
		if u.Below(w) {
			return true
		}
	}
	return false
}

func (u *unit) Above(v gomel.Unit) bool {
	return v.Below(u)
}
