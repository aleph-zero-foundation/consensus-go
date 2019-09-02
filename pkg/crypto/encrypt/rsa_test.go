package encrypt_test

import (
	"bytes"
	"encoding/binary"
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/crypto/encrypt"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

var _ = Describe("Encryption", func() {

	var (
		ek  gomel.EncryptionKey
		dk  gomel.DecryptionKey
		ct  gomel.CipherText
		err error
	)

	Describe("small", func() {

		BeforeEach(func() {
			ek, dk, _ = GenerateKeys()
		})

		Describe("Checking enc/dec", func() {

			var msg []byte

			BeforeEach(func() {
				msg = make([]byte, 8)
				binary.LittleEndian.PutUint64(msg, rand.Uint64())
				ct, err = ek.Encrypt(msg)
			})

			It("Should decrypt correctly", func() {
				Expect(err).To(BeNil())
				dmsg, err := dk.Decrypt(ct)
				Expect(err).To(BeNil())
				Expect(bytes.Equal(msg, dmsg)).To(BeTrue())
			})

			It("Should return false for forged ciphertext", func() {
				ct[0]++
				dmsg, err := dk.Decrypt(ct)
				Expect(err).NotTo(BeNil())
				Expect(bytes.Equal(msg, dmsg)).To(BeFalse())
			})
		})
		Describe("Checking encoding", func() {
			var text string
			BeforeEach(func() {
				text = ek.Encode()
			})

			It("Should decode correctly", func() {
				ekd, err := NewEncryptionKey(text)
				Expect(err).To(BeNil())
				Expect(eq(ek, ekd)).To(BeTrue())
			})
			It("Should throw an error for malformed data", func() {
				text = "x" + text[1:]
				_, err := NewEncryptionKey(text)
				Expect(err).NotTo(BeNil())
			})
		})
	})
})

func eq(ek1, ek2 gomel.EncryptionKey) bool {
	return ek1.Encode() == ek2.Encode()
}
