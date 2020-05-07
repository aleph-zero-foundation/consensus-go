package creator_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDag(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Creator Suite")
}
