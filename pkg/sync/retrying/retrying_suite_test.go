package retrying_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFallback(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Retrying Suite")
}
