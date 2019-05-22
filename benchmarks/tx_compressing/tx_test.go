package tx_compress_bench_test

import (
	. "gitlab.com/alephledger/consensus-go/pkg/transactions"

	"bufio"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"math/rand"
	"os"
	"strconv"
)

var _ = Describe("Tx", func() {
	var (
		data         []byte
		encoded      []byte
		decompressed []byte
		eErr         error
		dErr         error
		txs          []Tx
		txsDecoded   []Tx
		level        int
	)
	BeforeEach(func() {
		txs = generateRandom(10000)
	})
	Measure("Encoding/Decoding/Compression per level", func(b Benchmarker) {
		for level = 0; level < 10; level++ {
			data = Encode(txs)
			b.Time("level "+strconv.Itoa(level)+" compression time", func() {
				encoded, eErr = Compress(data, level)
			})
			Expect(eErr).NotTo(HaveOccurred())

			b.RecordValue("level "+strconv.Itoa(level)+" compression", float64(len(encoded))/float64(len(data)))

			b.Time("level "+strconv.Itoa(level)+" decompression time", func() {
				decompressed, dErr = Decompress(encoded)
			})
			Expect(dErr).NotTo(HaveOccurred())
			Expect(data).To(Equal(decompressed))
			txsDecoded, dErr = Decode(decompressed)
			Expect(txsDecoded).To(Equal(txs))
		}
	}, 5)

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
	users := readUsers("../../pkg/testdata/users.txt")
	var txID uint32
	for len(result) < howMany {
		result = append(result, Tx{
			ID:       txID,
			Issuer:   users[rand.Intn(len(users))],
			Receiver: users[rand.Intn(len(users))],
			Amount:   uint32(rand.Intn(1000000000)),
		})
		txID++
	}
	return result
}
