package bn256_test

import (
	. "gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Signing", func() {
	var (
		pub  *VerificationKey
		priv *SecretKey
	)
	BeforeEach(func() {
		var err error
		pub, priv, err = GenerateKeys()
		Expect(err).NotTo(HaveOccurred())
	})
	Context("Some data", func() {
		var data []byte
		BeforeEach(func() {
			data = []byte("19890604")
		})
		Describe("When signed", func() {
			var signature *Signature
			BeforeEach(func() {
				signature = priv.Sign(data)
			})
			It("should be successfully verified", func() {
				Expect(pub.Verify(signature, data)).To(BeTrue())
			})
			It("should be successfully verified after marshaling and unmarshaling the signature", func() {
				sgn, err := new(Signature).Unmarshal(signature.Marshal())
				Expect(err).NotTo(HaveOccurred())
				Expect(pub.Verify(sgn, data)).To(BeTrue())
			})
			It("should fail for different data", func() {
				Expect(pub.Verify(signature, []byte("19890535"))).To(BeFalse())
			})
		})
	})
})
