package custom_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCustom(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Custom Suite")
}
