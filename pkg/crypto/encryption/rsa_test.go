package encrypto_test

import (
    "binary"
    "bytes"
    "math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/crypto/encrypt"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Encryption", func() {

	var (
		ek gomel.EncryptionKey
		dk gomel.DecryptionKey
		ct gomel.CipherText
        err error
        msg []byte
	)

	Describe("small", func() {

		BeforeEach(func() {
			ek, dk, _ = GenerateKeys()
		})

		Describe("Checking enc/dec", func() {

			BeforeEach(func() {
				msg = binary.LittleEndian.Int64(rand.Int())
                ct, eerr = ek.Encrypt(msg)
			})

			It("Should decrypt correctly", func() {
                Expect(err).To(BeNil())
                dmsg, err := dk.Decrypt(ct)
                Expect(derr).To(BeNil())
				Expect(bytes.Equal(msg, dmsg).To(BeTrue())
			})

			It("Should return false for forged ciphertext", func() {
				ct[0]++
                dmsg, err := dk.Decrypt(ct)
                Expect(derr).NotTo(BeNil())
				Expect(bytes.Equal(msg, dmsg).To(BeFalse())
			})
		})
		Describe("Checking encoding", func() {
		})
	})

})
