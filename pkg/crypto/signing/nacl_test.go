package signing_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
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
		Describe("Decoding encoded public key", func() {
			It("Should return the key", func() {
				encoded := pub.Encode()
				decoded, err := DecodePublicKey(encoded)
				Expect(err).NotTo(HaveOccurred())
				Expect(decoded).To(Equal(pub))
			})
		})
		Describe("Decoding encoded private key", func() {
			It("Should return the key", func() {
				encoded := priv.Encode()
				decoded, err := DecodePrivateKey(encoded)
				Expect(err).NotTo(HaveOccurred())
				Expect(decoded).To(Equal(priv))
			})
		})
		Describe("Decoding non-base64", func() {
			It("Should return an error", func() {
				_, err := DecodePublicKey("abc*")
				Expect(err).To(HaveOccurred())
				_, err = DecodePrivateKey("abc*")
				Expect(err).To(HaveOccurred())
			})
		})
		Describe("Decoding public key as private key and vice versa", func() {
			It("Should return an error", func() {
				_, err := DecodePublicKey(priv.Encode())
				Expect(err).To(HaveOccurred())
				_, err = DecodePrivateKey(pub.Encode())
				Expect(err).To(HaveOccurred())
			})
		})
	})

})
