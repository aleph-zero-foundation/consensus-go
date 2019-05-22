package transactions_test

import (
	. "gitlab.com/alephledger/consensus-go/pkg/transactions"

	"bufio"
	"bytes"
	"encoding/binary"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"math/rand"
	"os"
)

var _ = Describe("Tx", func() {
	var (
		data        []byte
		dataDecoded []byte
		eErr        error
		dErr        error
		txs         []Tx
		txsDecoded  []Tx
	)
	BeforeEach(func() {
		txs = generateRandom(1000)
	})
	Describe("Encode & Decode", func() {
		Context("On empty transaction list", func() {
			BeforeEach(func() {
				data = Encode(txs[:0])
				txsDecoded, dErr = Decode(data)
			})
			It("Should run without errors", func() {
				Expect(dErr).NotTo(HaveOccurred())
			})
			It("Should return empty transaction list", func() {
				Expect(txsDecoded).To(BeEmpty())
			})
		})
		Context("On list of one transaction", func() {
			BeforeEach(func() {
				data = Encode(txs[:1])
				txsDecoded, dErr = Decode(data)
			})
			It("Should run without errors", func() {
				Expect(dErr).NotTo(HaveOccurred())
			})
			It("Should return a list containing the transaction", func() {
				Expect(txsDecoded).To(HaveLen(1))
				Expect(txsDecoded).To(Equal(txs[:1]))
			})
		})
		Context("On list of 1000 random transactions", func() {
			BeforeEach(func() {
				data = Encode(txs[:1000])
				txsDecoded, dErr = Decode(data)
			})
			It("Should run without errors", func() {
				Expect(dErr).NotTo(HaveOccurred())
			})
			It("Should return a list containing the transaction", func() {
				Expect(txsDecoded).To(HaveLen(1000))
				Expect(txsDecoded).To(Equal(txs[:1000]))
			})
		})
	})
	Describe("Decode", func() {
		Context("On data with missing fields", func() {
			BeforeEach(func() {
				var buf bytes.Buffer
				for _, tx := range txs[:10] {
					binary.Write(&buf, binary.LittleEndian, tx.ID)
					binary.Write(&buf, binary.LittleEndian, uint8(len(tx.Issuer)))
					binary.Write(&buf, binary.LittleEndian, uint8(len(tx.Receiver)))
					binary.Write(&buf, binary.LittleEndian, tx.Amount)
				}
				txsDecoded, dErr = Decode(buf.Bytes())
			})
			It("should return error", func() {
				Expect(dErr).To(HaveOccurred())
			})
		})
	})
	Describe("Compress & Decompress", func() {
		Context("On empty slice of data", func() {
			BeforeEach(func() {
				data, eErr = Compress([]byte{}, 9)
				dataDecoded, dErr = Decompress(data)
			})
			It("Should run without errors", func() {
				Expect(eErr).NotTo(HaveOccurred())
				Expect(dErr).NotTo(HaveOccurred())
			})
			It("Should return an empty slice of data", func() {
				Expect(dataDecoded).To(Equal([]byte{}))
			})
		})
		Context("On some non empty slice of data", func() {
			BeforeEach(func() {

				data, eErr = Compress([]byte("abcdef"), 5)
				dataDecoded, dErr = Decompress(data)
			})
			It("Should run without errors", func() {
				Expect(eErr).NotTo(HaveOccurred())
				Expect(dErr).NotTo(HaveOccurred())
			})
			It("Should return the same slice of data", func() {
				Expect(dataDecoded).To(Equal([]byte("abcdef")))
			})
		})
	})
	Describe("Decompress", func() {
		Context("On trash data", func() {
			BeforeEach(func() {
				data = []byte("fjdlsajfl")
				dataDecoded, dErr = Decompress(data)
			})
			It("should return error", func() {
				Expect(dErr).To(HaveOccurred())
			})
		})
	})
})

func readUsers(filename string) []string {
	file, _ := os.Open(filename)
	defer file.Close()
	scanner := bufio.NewScanner(file)
	users := []string{}
	for scanner.Scan() {
		users = append(users, scanner.Text())
	}
	return users
}

func generateRandom(howMany int) []Tx {
	result := []Tx{}
	users := readUsers("../testdata/users.txt")
	var txID uint32
	for len(result) < howMany {
		result = append(result, Tx{
			ID:       txID,
			Issuer:   users[rand.Intn(len(users))],
			Receiver: users[rand.Intn(len(users))],
			Amount:   uint32(rand.Intn(10)),
		})
		txID++
	}
	return result
}
