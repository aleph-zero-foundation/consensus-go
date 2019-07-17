package beacon_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBeacon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Beacon Suite")
}
