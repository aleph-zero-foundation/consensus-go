package random_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRandom(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Random Suite")
}
