package tcoin_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestTcoin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TCoin Suite")
}
