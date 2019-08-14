package fetch_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFetch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fetch Suite")
}
