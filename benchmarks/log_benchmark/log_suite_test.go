package logbench_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
	"testing"
)

var logfile = "benchmark.log"

var _ = AfterSuite(func() {
	os.Remove(logfile)
})

func TestBooks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Log speed benchmark")
}
