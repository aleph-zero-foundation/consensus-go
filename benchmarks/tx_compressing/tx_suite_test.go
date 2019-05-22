package tx_compress_bench_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLinear(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tx Benchmark Suite")
}
