package scalar_product_bench_test

import (
	"crypto/rand"
	"math/big"
	"strconv"
	"sync"

	"crypto/subtle"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudflare/bn256"
)

func simpleScalarProduct(nums []*big.Int, elems []*bn256.G2) *bn256.G2 {
	result := new(bn256.G2)
	temp := new(bn256.G2)
	for i, e := range elems {
		result.Add(result, temp.ScalarMult(e, nums[i]))
	}
	return result
}

func concurrentScalarProduct(nums []*big.Int, elems []*bn256.G2) *bn256.G2 {
	scalarProduct := new(bn256.G2)
	summands := make(chan *bn256.G2)

	var wg sync.WaitGroup
	for i, e := range elems {
		wg.Add(1)
		go func(e *bn256.G2, i int) {
			defer wg.Done()
			summands <- new(bn256.G2).ScalarMult(e, nums[i])
		}(e, i)
	}
	go func() {
		wg.Wait()
		close(summands)
	}()

	for summand := range summands {
		scalarProduct.Add(scalarProduct, summand)
	}
	return scalarProduct
}

var _ = Describe("Scalar Product", func() {
	var nTests = []int{256, 512, 1024}
	Measure("calculating sp", func(b Benchmarker) {
		for _, n := range nTests {
			nums := make([]*big.Int, n)
			elems := make([]*bn256.G2, n)

			for i := 0; i < n; i++ {
				r1, _ := rand.Int(rand.Reader, bn256.Order)
				nums[i] = r1
				r2, _ := rand.Int(rand.Reader, bn256.Order)
				elems[i] = new(bn256.G2).ScalarBaseMult(r2)
			}

			var res1, res2 *bn256.G2
			b.Time("simple loop n="+strconv.Itoa(n), func() {
				res1 = simpleScalarProduct(nums, elems)
			})
			b.Time("parallel n="+strconv.Itoa(n), func() {
				res2 = concurrentScalarProduct(nums, elems)
			})
			Expect(subtle.ConstantTimeCompare(res1.Marshal(), res2.Marshal()) == 1).To(BeTrue())
		}
	}, 10)
})
