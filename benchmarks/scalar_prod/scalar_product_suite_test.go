package scalar_product_bench_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestScalarProduct(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Benchmark Scalar Product")
}
