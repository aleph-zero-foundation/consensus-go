package signing_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Signatures", func() {

	var (
		pu   gomel.Preunit
		pub  gomel.PublicKey
		priv gomel.PrivateKey
		sig  gomel.Signature
	)

	Describe("small", func() {

		BeforeEach(func() {
			pub, priv, _ = GenerateKeys()
		})

		Describe("Checking signatures of preunits", func() {

			BeforeEach(func() {
				pu = tests.NewPreunit(0, []*gomel.Hash{}, []byte{}, nil)
				sig = priv.Sign(pu)
				pu.SetSignature(sig)
			})

			It("Should return true when checking by hand", func() {
				Expect(pub.Verify(pu)).To(BeTrue())
			})

			It("Should return false for forged signature", func() {
				sig[0]++
				pu.SetSignature(sig)
				Expect(pub.Verify(pu)).To(BeFalse())
			})
		})
	})

})
