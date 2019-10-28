package forking_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestForking(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Forking Suite")
}
