package creating_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCreating(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Creating Suite")
}
