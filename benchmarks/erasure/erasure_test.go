package erasure_bench_test

import (
	"math/rand"
	"strconv"

	"github.com/klauspost/reedsolomon"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

var _ = Describe("ReedSolomon", func() {
	var (
		nTests = []int{128, 256}
		szs    = []int{5, 10, 20, 50}
		f      int
		data   [][]byte
		cp     [][]byte
	)
	Measure("encoding/decoding", func(b Benchmarker) {
		for _, n := range nTests {
			for _, s := range szs {
				f = gomel.MinimalTrusted(n)
				size := (s*1024 + f - 1) / f
				data = make([][]byte, n)
				cp = make([][]byte, n)
				for i := 0; i < f; i++ {
					data[i] = make([]byte, size)
					rand.Read(data[i])
				}
				for i := f; i < n; i++ {
					data[i] = make([]byte, size)
				}

				enc, _ := reedsolomon.New(f, n-f)
				b.Time("encoding time n="+strconv.Itoa(n)+" unit size="+strconv.Itoa(s)+"kB shrad size="+strconv.Itoa(size)+"B", func() {
					enc.Encode(data)
				})
				for i := 0; i < n; i++ {
					cp[i] = make([]byte, size)
					copy(cp[i], data[i])
				}
				shards := rand.Perm(n)
				for i := 0; i < n-f; i++ {
					data[shards[i]] = nil
				}
				b.Time("reconstruction time n="+strconv.Itoa(n)+" unit size="+strconv.Itoa(s)+"kB shrad size="+strconv.Itoa(size)+"B", func() {
					enc.Reconstruct(data)
				})
				for i := 0; i < n; i++ {
					for j := 0; j < size; j++ {
						Expect(data[i][j]).To(Equal(cp[i][j]))
					}
				}
			}
		}
	}, 10)

})
