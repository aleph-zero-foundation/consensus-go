package growing_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGrowing(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Growing Suite")
}
