package erasure_bench_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLinear(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Benchmark Erasure")
}
