package signing_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
)

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

var _ = Describe("Signatures", func() {

	var (
		pu   *preunit
		pub  PublicKey
		priv PrivateKey
	)

	Describe("small", func() {

		BeforeEach(func() {
			pub, priv, _ = GenerateKeys()
		})

		Describe("Checking signatures of preunits", func() {

			BeforeEach(func() {
				pu = &preunit{}
				pu.hash[0] = 1
				pu.signature = priv.Sign(pu)
			})

			It("Should return true when checking by hand", func() {
				Expect(pub.Verify(pu)).To(BeTrue())
			})

			It("Should return false for forged signature", func() {
				pu.signature[0]++
				Expect(pub.Verify(pu)).To(BeFalse())
			})
		})
	})

})
